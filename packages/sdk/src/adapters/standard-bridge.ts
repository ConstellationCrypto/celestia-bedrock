/* eslint-disable @typescript-eslint/no-unused-vars */
import {
  ethers,
  Contract,
  Overrides,
  Signer,
  BigNumber,
  CallOverrides,
} from 'ethers'
import {
  TransactionRequest,
  TransactionResponse,
  BlockTag,
} from '@ethersproject/abstract-provider'
import { predeploys } from '@eth-optimism/contracts'
import { hexStringEquals } from '@eth-optimism/core-utils'

import l1StandardBridgeArtifact from '../forge-artifacts/L1StandardBridge.json'
import l2StandardBridgeArtifact from '../forge-artifacts/L2StandardBridge.json'
import optimismMintableERC20 from '../forge-artifacts/OptimismMintableERC20.json'
import IERC20 from '../forge-artifacts/IERC20.json'
import { CrossChainMessenger } from '../cross-chain-messenger'
import {
  IBridgeAdapter,
  NumberLike,
  AddressLike,
  TokenBridgeMessage,
  MessageDirection,
} from '../interfaces'
import { toAddress, omit } from '../utils'

/**
 * Bridge adapter for any token bridge that uses the standard token bridge interface.
 */
export class StandardBridgeAdapter implements IBridgeAdapter {
  public messenger: CrossChainMessenger
  public l1Bridge: Contract
  public l2Bridge: Contract

  /**
   * Creates a StandardBridgeAdapter instance.
   *
   * @param opts Options for the adapter.
   * @param opts.messenger Provider used to make queries related to cross-chain interactions.
   * @param opts.l1Bridge L1 bridge contract.
   * @param opts.l2Bridge L2 bridge contract.
   */
  constructor(opts: {
    messenger: CrossChainMessenger
    l1Bridge: AddressLike
    l2Bridge: AddressLike
  }) {
    const wrap = (x) =>
      new ethers.utils.Interface(x)
        .format()
        .concat(
          'function REMOTE_TOKEN() view returns(address)',
          'function LOCAL_TOKEN() view returns(address)'
        )
    this.messenger = opts.messenger
    this.l1Bridge = new Contract(
      toAddress(opts.l1Bridge),
      wrap(l1StandardBridgeArtifact.abi),
      this.messenger.l1Provider
    )
    this.l2Bridge = new Contract(
      toAddress(opts.l2Bridge),
      wrap(l2StandardBridgeArtifact.abi),
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
    const events = await this.l1Bridge.queryFilter(
      this.l1Bridge.filters.ERC20DepositInitiated(
        undefined,
        undefined,
        address
      ),
      opts?.fromBlock,
      opts?.toBlock
    )

    return events
      .filter((event) => {
        // Specifically filter out ETH. ETH deposits and withdrawals are handled by the ETH bridge
        // adapter. Bridges that are not the ETH bridge should not be able to handle or even
        // present ETH deposits or withdrawals.
        // Exception: ETH is bridged to L1_ETH if FPE is enabled.
        return (
          // !hexStringEquals(event.args.l1Token, ethers.constants.AddressZero) &&
          !hexStringEquals(event.args.l2Token, predeploys.OVM_ETH)
        )
      })
      .map((event) => {
        return {
          direction: MessageDirection.L1_TO_L2,
          from: event.args.from,
          to: event.args.to,
          l1Token: event.args.l1Token,
          l2Token: event.args.l2Token,
          amount: event.args.amount,
          data: event.args.extraData,
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
    const events = await this.l2Bridge.queryFilter(
      this.l2Bridge.filters.WithdrawalInitiated(undefined, undefined, address),
      opts?.fromBlock,
      opts?.toBlock
    )

    return events
      .filter((event) => {
        // Specifically filter out ETH. ETH deposits and withdrawals are handled by the ETH bridge
        // adapter. Bridges that are not the ETH bridge should not be able to handle or even
        // present ETH deposits or withdrawals.
        // Exception: L1_ETH withdraws to ETH if FPE is enabled.
        return (
          // !hexStringEquals(event.args.l1Token, ethers.constants.AddressZero) &&
          !hexStringEquals(event.args.l2Token, predeploys.OVM_ETH)
        )
      })
      .map((event) => {
        return {
          direction: MessageDirection.L2_TO_L1,
          from: event.args.from,
          to: event.args.to,
          l1Token: event.args.l1Token,
          l2Token: event.args.l2Token,
          amount: event.args.amount,
          data: event.args.extraData,
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

  public async supportsTokenPair(
    l1Token: AddressLike,
    l2Token: AddressLike
  ): Promise<boolean> {
    return !!(await this.nativeChain(l1Token, l2Token))
  }

  // return 0 if unsupported, 1 if the token is native to the l1, 2 if the token is native to the l2.
  public async nativeChain(
    l1Token: AddressLike,
    l2Token: AddressLike
  ): Promise<0 | 1 | 2> {
    try {
      const l1Zero = hexStringEquals(
        toAddress(l1Token),
        ethers.constants.AddressZero
      )
      const l2Zero = hexStringEquals(
        toAddress(l2Token),
        ethers.constants.AddressZero
      )

      if (
        (l1Zero && l2Zero) ||
        hexStringEquals(toAddress(l2Token), predeploys.OVM_ETH)
      ) {
        return 0
      }

      // Can use either the l1 or l2 bridge - both store the token information.
      if (l2Zero) {
        // Make sure the L1 token matches
        try {
          return hexStringEquals(
            await this.l1Bridge.LOCAL_TOKEN(),
            toAddress(l1Token)
          )
            ? 1
            : 0
        } catch (error) {
          // LOCAL_TOKEN() may not exist
          if (error?.code !== 'CALL_EXCEPTION') {
            console.error('Unexpected err when fetching local token', error)
            throw error
          }
          return 0
        }
      }

      if (l1Zero) {
        // Make sure the L2 token matches
        try {
          return hexStringEquals(
            await this.l1Bridge.REMOTE_TOKEN(),
            toAddress(l2Token)
          )
            ? 1
            : 0
        } catch (error) {
          // REMOTE_TOKEN() may not exist
          if (error?.code !== 'CALL_EXCEPTION') {
            console.error('Unexpected err when fetching remote token', error)
            throw error
          }
          return 0
        }
      }

      // Don't support ETH deposits or withdrawals via this bridge.
      if (hexStringEquals(toAddress(l2Token), predeploys.OVM_ETH)) {
        return 0
      }

      try {
        const l2Contract = new Contract(
          toAddress(l2Token),
          optimismMintableERC20.abi,
          this.messenger.l2Provider
        )

        // Make sure the L1 token matches.
        const remoteL1Token = await l2Contract.l1Token()

        if (!hexStringEquals(remoteL1Token, toAddress(l1Token))) {
          return 0
        }

        // Make sure the L2 bridge matches.
        const remoteL2Bridge = await l2Contract.l2Bridge()
        if (!hexStringEquals(remoteL2Bridge, this.l2Bridge.address)) {
          return 0
        }

        return 1
      } catch (err) {
        if (err?.code !== 'CALL_EXCEPTION') {
          console.error('Unexpected err when checking bridge', err)
          throw err
        }
      }

      const contract = new Contract(
        toAddress(l1Token),
        optimismMintableERC20.abi,
        this.messenger.l1Provider
      )

      // Make sure the L2 token matches.
      const remoteL2Token = await contract.REMOTE_TOKEN()

      if (!hexStringEquals(remoteL2Token, toAddress(l2Token))) {
        return 0
      }

      // Make sure the L1 bridge matches.
      const remoteL1Bridge = await contract.BRIDGE()
      if (!hexStringEquals(remoteL1Bridge, this.l1Bridge.address)) {
        return 0
      }

      return 2
    } catch (err) {
      // If the L2 token is not an L2StandardERC20, it may throw an error. If there's a call
      // exception then we assume that the token is not supported. Other errors are thrown. Since
      // the JSON-RPC API is not well-specified, we need to handle multiple possible error codes.
      if (
        /*
        !err?.message?.toString().includes('CALL_EXCEPTION') &&
        !err?.stack?.toString().includes('execution reverted')
        */
        err?.code !== 'CALL_EXCEPTION'
      ) {
        console.error('Unexpected error when checking bridge', err)
        throw err
      }
      return 0
    }
  }

  public async approval(
    l1Token: AddressLike,
    l2Token: AddressLike,
    signer: ethers.Signer
  ): Promise<BigNumber> {
    const chain = await this.nativeChain(l1Token, l2Token)

    if (!chain) {
      throw new Error(`token pair not supported by bridge`)
    }

    const token = new Contract(
      toAddress(chain === 1 ? l1Token : l2Token),
      IERC20.abi,
      chain === 1 ? this.messenger.l1Provider : this.messenger.l2Provider
    )
    return token.allowance(
      await signer.getAddress(),
      chain === 1 ? this.l1Bridge.address : this.l2Bridge.address
    )
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
      const chain = await this.nativeChain(l1Token, l2Token)
      if (!chain) {
        throw new Error(`token pair not supported by bridge`)
      }

      const token = new Contract(
        toAddress(chain === 1 ? l1Token : l2Token),
        IERC20.abi,
        chain === 1 ? this.messenger.l1Provider : this.messenger.l2Provider
      )

      return token.populateTransaction.approve(
        chain === 1 ? this.l1Bridge.address : this.l2Bridge.address,
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

      if (opts?.recipient === undefined) {
        return this.l1Bridge.populateTransaction.depositERC20(
          toAddress(l1Token),
          toAddress(l2Token),
          amount,
          opts?.l2GasLimit || 200_000, // Default to 200k gas limit.
          '0x', // No data.
          {
            ...omit(opts?.overrides || {}, 'value'),
            value: hexStringEquals(
              toAddress(l1Token),
              ethers.constants.AddressZero
            )
              ? amount
              : 0,
          }
        )
      } else {
        return this.l1Bridge.populateTransaction.depositERC20To(
          toAddress(l1Token),
          toAddress(l2Token),
          toAddress(opts.recipient),
          amount,
          opts?.l2GasLimit || 200_000, // Default to 200k gas limit.
          '0x', // No data.
          {
            ...omit(opts?.overrides || {}, 'value'),
            value: hexStringEquals(
              toAddress(l1Token),
              ethers.constants.AddressZero
            )
              ? amount
              : 0,
          }
        )
      }
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
      const chain = await this.nativeChain(l1Token, l2Token)
      if (!chain) {
        throw new Error(`token pair not supported by bridge`)
      }

      if (chain === 1) {
        // use legacy withdraw method for max compatibility
        if (opts?.recipient === undefined) {
          return this.l2Bridge.populateTransaction.withdraw(
            toAddress(l2Token),
            amount,
            0, // L1 gas not required.
            '0x', // No data.
            {
              ...omit(opts?.overrides || {}, 'value'),
              value: hexStringEquals(
                toAddress(l2Token),
                ethers.constants.AddressZero
              )
                ? amount
                : 0,
            }
          )
        } else {
          return this.l2Bridge.populateTransaction.withdrawTo(
            toAddress(l2Token),
            toAddress(opts.recipient),
            amount,
            0, // L1 gas not required.
            '0x', // No data.
            {
              ...omit(opts?.overrides || {}, 'value'),
              value: hexStringEquals(
                toAddress(l2Token),
                ethers.constants.AddressZero
              )
                ? amount
                : 0,
            }
          )
        }
      }

      if (opts?.recipient === undefined) {
        return this.l2Bridge.populateTransaction.bridgeERC20(
          toAddress(l2Token),
          toAddress(l1Token),
          amount,
          0, // L1 gas not required.
          '0x', // No data.
          {
            ...omit(opts?.overrides || {}, 'value'),
            value: hexStringEquals(
              toAddress(l2Token),
              ethers.constants.AddressZero
            )
              ? amount
              : 0,
          }
        )
      } else {
        return this.l2Bridge.populateTransaction.bridgeERC20To(
          toAddress(l2Token),
          toAddress(l1Token),
          toAddress(opts.recipient),
          amount,
          0, // L1 gas not required.
          '0x', // No data.
          {
            ...omit(opts?.overrides || {}, 'value'),
            value: hexStringEquals(
              toAddress(l2Token),
              ethers.constants.AddressZero
            )
              ? amount
              : 0,
          }
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
