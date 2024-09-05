/* eslint-disable @typescript-eslint/no-unused-vars */
import {
  ethers,
  Contract,
  Overrides,
  Signer,
  BigNumber,
  CallOverrides,
  Bytes,
} from 'ethers'
import {
  TransactionRequest,
  TransactionResponse,
  BlockTag,
} from '@ethersproject/abstract-provider'
import { predeploys } from '@eth-optimism/contracts'
import { hexStringEquals } from '@eth-optimism/core-utils'

import l2ToL1MessagePasser from '../forge-artifacts/L2ToL1MessagePasser.json'
import l2StandardBridgeArtifact from '../forge-artifacts/L2StandardBridge.json'
import optimismMintableERC20 from '../forge-artifacts/OptimismMintableERC20.json'
import l1OptimismPortalArtifact from '../forge-artifacts/OptimismPortal.json'
import l1SystemConfigArtifact from '../forge-artifacts/SystemConfig.json'
import { CrossChainMessenger } from '../cross-chain-messenger'
import {
  IBridgeAdapter,
  NumberLike,
  AddressLike,
  TokenBridgeMessage,
  MessageDirection,
} from '../interfaces'
import { opaqueDataToDepositData, toAddress } from '../utils'

/**
 * Bridge adapter for any token bridge that uses the standard token bridge interface.
 */
export class OptimismPortalBridgeAdapter implements IBridgeAdapter {
  public messenger: CrossChainMessenger
  public l1Bridge: Contract
  public l2Bridge: Contract
  public l1SystemConfig?: Contract
  public l2ToL1MessagePasser: Contract

  /**
   * Creates a StandardBridgeAdapter instance.
   *
   * @param opts Options for the adapter.
   * @param opts.messenger Provider used to make queries related to cross-chain interactions.
   * @param opts.l1Bridge L1 bridge contract.
   * @param opts.l2Bridge L2 bridge contract.
   * @param opts.l1SystemConfig L1 System Config contract.
   */
  constructor(opts: {
    messenger: CrossChainMessenger
    l1Bridge: AddressLike
    l2Bridge: AddressLike
    l1SystemConfig?: AddressLike
  }) {
    this.messenger = opts.messenger
    this.l1Bridge = new Contract(
      toAddress(opts.l1Bridge),
      l1OptimismPortalArtifact.abi,
      this.messenger.l1Provider
    )
    this.l2Bridge = new Contract(
      toAddress(opts.l2Bridge),
      l2StandardBridgeArtifact.abi,
      this.messenger.l2Provider
    )
    if (opts.l1SystemConfig) {
      this.l1SystemConfig = new Contract(
        toAddress(opts.l1SystemConfig),
        l1SystemConfigArtifact.abi,
        this.messenger.l1Provider
      )
    }

    this.l2ToL1MessagePasser = new Contract(
      toAddress('0x4200000000000000000000000000000000000016'),
      l2ToL1MessagePasser.abi,
      this.messenger.l2Provider
    )
  }

  public async getDepositsByAddress(
    address: AddressLike,
    opts?: {
      fromBlock?: BlockTag
      toBlock?: BlockTag
    }
  ): Promise<TokenBridgeMessage[]> {
    const token = (await this.l1SystemConfig.gasPayingToken()).addr_
    const events = await this.l1Bridge.queryFilter(
      this.l1Bridge.filters.TransactionDeposited(
        address,
        undefined,
        undefined,
        undefined
      ),
      opts?.fromBlock,
      opts?.toBlock
    )
    return events
      .map((event) => {
        const obj = opaqueDataToDepositData(event.args.opaqueData)
        return {
          direction: MessageDirection.L1_TO_L2,
          from: event.args.from,
          to: event.args.to,
          l1Token: token,
          l2Token: token,
          amount: obj['mint'],
          data: 'PortalBridge' + event.args.opaqueData,
          logIndex: event.logIndex,
          blockNumber: event.blockNumber,
          transactionHash: event.transactionHash,
        }
      })
      .sort((a, b) => {
        // Sort descending by block number
        return b.blockNumber - a.blockNumber
      })
  }

  public async getWithdrawalsByAddress(
    address: AddressLike,
    opts?: {
      fromBlock?: BlockTag
      toBlock?: BlockTag
    }
  ): Promise<TokenBridgeMessage[]> {
    const token = (await this.l1SystemConfig.gasPayingToken()).addr_

    const events = await this.l2ToL1MessagePasser.queryFilter(
      this.l2ToL1MessagePasser.filters.MessagePassed(
        undefined,
        address,
        undefined,
        undefined,
        undefined,
        undefined
      ),
      opts?.fromBlock,
      opts?.toBlock
    )
    return (
      events
        // .filter((event) => {
        //   // Specifically filter out ETH. ETH deposits and withdrawals are handled by the ETH bridge
        //   // adapter. Bridges that are not the ETH bridge should not be able to handle or even
        //   // present ETH deposits or withdrawals.
        //   return (
        //     !hexStringEquals(event.args.l1Token, ethers.constants.AddressZero) &&
        //     !hexStringEquals(event.args.l2Token, predeploys.OVM_ETH)
        //   )
        // })
        .map((event) => {
          return {
            direction: MessageDirection.L2_TO_L1,
            from: event.args.sender,
            to: event.args.target,
            l1Token: token,
            l2Token: '0x4200000000000000000000000000000000000006',
            amount: event.args.value,
            data: event.args.data,
            logIndex: event.logIndex,
            blockNumber: event.blockNumber,
            transactionHash: event.transactionHash,
          }
        })
        .sort((a, b) => {
          // Sort descending by block number
          return b.blockNumber - a.blockNumber
        })
    )
  }

  public async supportsTokenPair(
    l1Token: AddressLike,
    l2Token: AddressLike
  ): Promise<boolean> {
    const gasPayingToken = (await this.l1SystemConfig.gasPayingToken()).addr_
    return hexStringEquals(toAddress(l1Token), gasPayingToken) //&&
    //hexStringEquals(toAddress(l2Token), predeploys.OVM_ETH) //this isn't true because wrapped erc20 fee token is 0x4200000000000000000000000000000000000006
  }

  public async approval(
    l1Token: AddressLike,
    l2Token: AddressLike,
    signer: ethers.Signer
  ): Promise<BigNumber> {
    if (!(await this.supportsTokenPair(l1Token, l2Token))) {
      throw new Error(`token pair not supported by bridge`)
    }

    const token = new Contract(
      toAddress(l1Token),
      optimismMintableERC20.abi,
      this.messenger.l1Provider
    )
    return token.allowance(await signer.getAddress(), this.l1Bridge.address)
  }

  public async approve(
    l1Token: AddressLike,
    l2Token: AddressLike,
    amount: NumberLike,
    signer: Signer,
    opts?: {
      overrides?: Overrides
    }
  ): Promise<TransactionResponse> {
    return signer.sendTransaction(
      await this.populateTransaction.approve(l1Token, l2Token, amount, opts)
    )
  }

  public async deposit(
    l1Token: AddressLike,
    l2Token: AddressLike,
    amount: NumberLike,
    signer: Signer,
    opts?: {
      recipient?: AddressLike
      l2GasLimit?: NumberLike
      overrides?: Overrides
    }
  ): Promise<TransactionResponse> {
    return signer.sendTransaction(
      await this.populateTransaction.deposit(l1Token, l2Token, amount, opts)
    )
  }

  public async withdraw(
    l1Token: AddressLike,
    l2Token: AddressLike,
    amount: NumberLike,
    signer: Signer,
    opts?: {
      recipient?: AddressLike
      overrides?: Overrides
    }
  ): Promise<TransactionResponse> {
    return signer.sendTransaction(
      await this.populateTransaction.withdraw(l1Token, l2Token, amount, opts)
    )
  }

  populateTransaction = {
    approve: async (
      l1Token: AddressLike,
      l2Token: AddressLike,
      amount: NumberLike,
      opts?: {
        overrides?: Overrides
      }
    ): Promise<TransactionRequest> => {
      if (!(await this.supportsTokenPair(l1Token, l2Token))) {
        throw new Error(`token pair not supported by bridge`)
      }

      const token = new Contract(
        toAddress(l1Token),
        optimismMintableERC20.abi,
        this.messenger.l1Provider
      )
      return token.populateTransaction.approve(
        this.l1Bridge.address,
        amount,
        opts?.overrides || {}
      )
    },

    deposit: async (
      l1Token: AddressLike,
      l2Token: AddressLike,
      amount: NumberLike,
      opts?: {
        recipient?: AddressLike
        l2GasLimit?: NumberLike
        overrides?: Overrides
      }
    ): Promise<TransactionRequest> => {
      if (!(await this.supportsTokenPair(l1Token, l2Token))) {
        throw new Error(`token pair not supported by bridge`)
      }

      return this.l1Bridge.populateTransaction.depositERC20Transaction(
        toAddress(opts.recipient),
        amount,
        amount,
        opts?.l2GasLimit || 200_000, // Default to 200k gas limit.
        false,
        '0x', // No data.
        opts?.overrides || {}
      )
    },

    withdraw: async (
      l1Token: AddressLike,
      l2Token: AddressLike,
      amount: NumberLike,
      opts?: {
        recipient?: AddressLike
        overrides?: Overrides
      }
    ): Promise<TransactionRequest> => {
      if (!(await this.supportsTokenPair(l1Token, l2Token))) {
        throw new Error(`token pair not supported by bridge`)
      }

      if (opts?.recipient === undefined) {
        return this.l2Bridge.populateTransaction.withdraw(
          toAddress(l2Token),
          amount,
          0, // L1 gas not required.
          '0x', // No data.
          opts?.overrides || {}
        )
      } else {
        return this.l2Bridge.populateTransaction.withdrawTo(
          toAddress(l2Token),
          toAddress(opts.recipient),
          amount,
          0, // L1 gas not required.
          '0x', // No data.
          opts?.overrides || {}
        )
      }
    },
  }

  estimateGas = {
    approve: async (
      l1Token: AddressLike,
      l2Token: AddressLike,
      amount: NumberLike,
      opts?: {
        overrides?: CallOverrides
      }
    ): Promise<BigNumber> => {
      return this.messenger.l1Provider.estimateGas(
        await this.populateTransaction.approve(l1Token, l2Token, amount, opts)
      )
    },

    deposit: async (
      l1Token: AddressLike,
      l2Token: AddressLike,
      amount: NumberLike,
      opts?: {
        recipient?: AddressLike
        l2GasLimit?: NumberLike
        overrides?: CallOverrides
      }
    ): Promise<BigNumber> => {
      return this.messenger.l1Provider.estimateGas(
        await this.populateTransaction.deposit(l1Token, l2Token, amount, opts)
      )
    },

    withdraw: async (
      l1Token: AddressLike,
      l2Token: AddressLike,
      amount: NumberLike,
      opts?: {
        recipient?: AddressLike
        overrides?: CallOverrides
      }
    ): Promise<BigNumber> => {
      return this.messenger.l2Provider.estimateGas(
        await this.populateTransaction.withdraw(l1Token, l2Token, amount, opts)
      )
    },
  }
}
