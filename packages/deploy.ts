import {
  writeFileSync,
  readFileSync,
  createWriteStream,
  copyFileSync,
} from 'node:fs'
import { execSync } from 'node:child_process'
import { Readable } from 'node:stream'
import { pipeline } from 'node:stream/promises'
import { randomBytes } from 'node:crypto'

import { ethers } from 'ethers'
import { emptyDirSync, copySync } from 'fs-extra'
import {
  S3Client,
  ListObjectsCommand,
  PutObjectCommand,
  GetObjectCommand,
} from '@aws-sdk/client-s3'

const main = async () => {
  let hasS3Data = false
  let S3_BUCKET: string
  let S3_PREFIX: string
  let s3: S3Client
  if (process.env.S3_FOLDER) {
    s3 = new S3Client({})
    const S3_FOLDER = process.env.S3_FOLDER + '/'
    if (S3_FOLDER.slice(0, 5) !== 's3://' || S3_FOLDER.slice(-2) === '//') {
      throw Error('Invalid S3_FOLDER url: ' + S3_FOLDER)
    }
    S3_BUCKET = S3_FOLDER.split('/')[2]
    S3_PREFIX = S3_FOLDER.split('/').slice(3).join('/')
    const command = new ListObjectsCommand({
      Bucket: S3_BUCKET,
      Prefix: S3_PREFIX,
    })
    const response = await s3.send(command)
    if (response.Contents) {
      hasS3Data = true
    }
  } else {
    console.warn('S3_FOLDER is missing - not uploading deployment to s3')
  }

  if (hasS3Data) {
    console.log('Downloading from s3 bucket')
    // try to copy rollup.json, contracts.json genesis.json from s3 bucket
    for (const file of ['rollup.json', 'contracts.json', 'genesis.json']) {
      const command = new GetObjectCommand({
        Bucket: S3_BUCKET,
        Key: S3_PREFIX + file,
      })
      const response = await s3.send(command)
      await pipeline(response.Body as Readable, createWriteStream(file))
    }
  } else {
    console.log('Deploying contracts')
    const DEPLOYER = process.env.DEPLOYER_ADDRESS
    const ADMIN = process.env.ADMIN_ADDRESS
    const L1_FEE_WALLET_ADDRESS = process.env.FEE_WALLET_ADDRESS || ADMIN
    const PROPOSER = process.env.PROPOSER_ADDRESS
    const BATCHER = process.env.BATCHER_ADDRESS
    const SEQUENCER = process.env.SEQUENCER_ADDRESS
    const L1_RPC = process.env.L1_RPC

    const provider = new ethers.providers.JsonRpcProvider(L1_RPC)
    const block = await provider.getBlock(process.env.L1_BLOCK_NUMBER ?? 'safe')
    const BLOCKHASH = block.hash
    const TIMESTAMP = block.timestamp

    const baseFeeVaultWithdrawalNetwork = 0; // 0 = L1, 1 = L2
    const l1FeeVaultWithdrawalNetwork = 0; // 0 = L1, 1 = L2
    const sequencerFeeVaultWithdrawalNetwork = 0; // 0 = L1, 1 = L2

    // https://github.com/ethereum-optimism/optimism/blob/develop/packages/contracts-bedrock/deploy-config/mainnet.json#L40C38-L40C44
    // https://docs.optimism.io/builders/chain-operators/management/configuration
    // https://docs.google.com/spreadsheets/d/12VIiXHaVECG2RUunDSVJpn67IQp9NHFJqUsma2PndpE/edit#gid=186414307
    // below defaults are for 75,000k transactions per day
    let gasPriceOracleBlobBaseFeeScalar
    let gasPriceOracleBaseFeeScalar
    if (process.env.DATA_AVAILABILITY_TYPE === "blobs") {
      gasPriceOracleBlobBaseFeeScalar = Number(process.env.GAS_PRICE_ORACLE_BLOB_BASE_FEE_SCALAR) || 659851
      gasPriceOracleBaseFeeScalar =  Number(process.env.GAS_PRICE_ORACLE_BASE_FEE_SCALAR) || 1101
    } else {
      gasPriceOracleBlobBaseFeeScalar = Number(process.env.GAS_PRICE_ORACLE_BLOB_BASE_FEE_SCALAR) || 0
      gasPriceOracleBaseFeeScalar = Number(process.env.GAS_PRICE_ORACLE_BASE_FEE_SCALAR) || 668098
    }

    // see op-chain-ops/genesis/config.go for documentation
    const json = {
      superchainConfigGuardian: L1_FEE_WALLET_ADDRESS,
      finalSystemOwner: L1_FEE_WALLET_ADDRESS,

      l1StartingBlockTag: BLOCKHASH,
      l1ChainID: Number(process.env.CHAIN_ID),
      l1BlockTime: Number(process.env.L1_BLOCK_TIME) || 12,
      l2ChainID: Number(process.env.L2_CHAIN_ID),
      l2BlockTime: Number(process.env.L2_BLOCK_TIME) || 2,

      maxSequencerDrift: Number(process.env.MAX_SEQUENCER_DRIFT) || 3600, // 1 hour max sequencer drift
      sequencerWindowSize: Number(process.env.SEQUENCER_WINDOW_SIZE) || 21600, // 72 hours
      channelTimeout: Number(process.env.CHANNEL_TIMEOUT) || 300, // 300 l1 blocks

      p2pSequencerAddress: SEQUENCER,
      batchInboxAddress: '0x' + randomBytes(20).toString('hex'),
      batchSenderAddress: BATCHER,

      l2OutputOracleSubmissionInterval: Number(process.env.L2_OUTPUT_ORACLE_SUBMISSION_INTERVAL) || 900, // 30 mins
      l2OutputOracleStartingTimestamp: TIMESTAMP,
      l2OutputOracleStartingBlockNumber: 0,

      l2OutputOracleProposer: PROPOSER,
      l2OutputOracleChallenger: ADMIN,

      finalizationPeriodSeconds: Number(
        process.env.FINALIZATION_PERIOD_SECONDS
      ), // 12, 604800

      proxyAdminOwner: ADMIN,
      baseFeeVaultRecipient: baseFeeVaultWithdrawalNetwork === 0 ? L1_FEE_WALLET_ADDRESS : ADMIN,
      l1FeeVaultRecipient: l1FeeVaultWithdrawalNetwork === 0 ? L1_FEE_WALLET_ADDRESS : ADMIN,
      sequencerFeeVaultRecipient: l1FeeVaultWithdrawalNetwork === 0 ? L1_FEE_WALLET_ADDRESS : ADMIN,

      baseFeeVaultMinimumWithdrawalAmount: '0xde0b6b3a7640000', // 1 ETH
      l1FeeVaultMinimumWithdrawalAmount: '0xde0b6b3a7640000', // 1 ETH
      sequencerFeeVaultMinimumWithdrawalAmount: '0xde0b6b3a7640000', // 1 ETH
      baseFeeVaultWithdrawalNetwork,
      l1FeeVaultWithdrawalNetwork,
      sequencerFeeVaultWithdrawalNetwork,

      gasPriceOracleBaseFeeScalar: gasPriceOracleBaseFeeScalar,
      gasPriceOracleBlobBaseFeeScalar: gasPriceOracleBlobBaseFeeScalar,

      gasPriceOracleOverhead: 2100,
      gasPriceOracleScalar: 0,

      enableGovernance: false, // do not predeploy the governance token onto the l2
      governanceTokenName: 'Optimism', // unused
      governanceTokenSymbol: 'OP', // unused
      governanceTokenOwner: ADMIN, // unused

      l2GenesisBlockGasLimit: '0x1c9c380', // 30,000,000 gas
      l2GenesisBlockBaseFeePerGas: '0x3b9aca00', // 1 gwei

      eip1559Denominator: Number(process.env.EIP1559Denominator) || 50,
      eip1559DenominatorCanyon: Number(process.env.EIP1559DenominatorCanyon) || 250,
      eip1559Elasticity: Number(process.env.EIP1559Elasticity) || 6,

      l2GenesisRegolithTimeOffset: '0x0', // seconds after genesis block that Regolith hard fork activates
      l2GenesisCanyonTimeOffset: '0x0',
      l2GenesisDeltaTimeOffset: '0x0',
      l2GenesisEcotoneTimeOffset: '0x0',
      l2GenesisFjordTimeOffset: '0x0',
      l2GenesisInteropTimeOffset: undefined,

      systemConfigStartBlock: 0,
      requiredProtocolVersion: "0x0000000000000000000000000000000000000000000000000000000000000000",
      recommendedProtocolVersion: "0x0000000000000000000000000000000000000000000000000000000000000000",

      fundDevAccounts: false,

      faultGameAbsolutePrestate: "0x035ac9f319e41b6dc184bf1153c9dbaead5d1e89c5ecc4212808ff5cc8f33b08", // ??
      faultGameMaxDepth: 73, // ??
      faultGameClockExtension: Number(process.env.FAULT_GAME_CLOCK_EXTENSION) || 120, // 2 mins
      faultGameMaxClockDuration: Number(process.env.FAULT_GAME_MAX_DURATION) || 1200, // 20 mins,
      faultGameGenesisBlock: 0,
      faultGameGenesisOutputRoot: "0x0000000000000000000000000000000000000000000000000000000000000000",
      faultGameSplitDepth: 32, // ??
      faultGameWithdrawalDelay: Number(process.env.FAULT_GAME_WITHDRAWAL_DELAY) || 1200, // 20 mins,
      preimageOracleMinProposalSize: 1800000, // ??
      preimageOracleChallengePeriod: Number(process.env.PREIMAGE_ORACLE_CHALLENGE_PERIOD) || 120, // 2 minutes

      proofMaturityDelaySeconds: Number(process.env.PROOF_MATURITY_DELAY_SECONDS) || 12,
      disputeGameFinalityDelaySeconds: Number(process.env.DISPUTE_GAME_FINALITY_DELAY_SECONDS) || 6,
      respectedGameType: 0,
      useFaultProofs: process.env.USE_FAULT_PROOFS === "true",

      useCustomGasToken: process.env.L1_FPE_TOKEN !== ethers.constants.AddressZero,
      customGasTokenAddress: process.env.L1_FPE_TOKEN,

      usePlasma: process.env.PLASMA === "true",
      //is not used if PLASMA=false
      daChallengeProxy: "0x0000000000000000000000000000000000000000",
      daCommitmentType: "GenericCommitment",
      daChallengeWindow: 300,
      daResolveWindow: 300
    }
    writeFileSync('deploy-config/deployer.json', JSON.stringify(json, null, 2))
    execSync(`DEPLOYMENT_OUTFILE=deployments/deployer/.deploy DEPLOYMENT_CONTEXT=deployer DEPLOY_CONFIG_PATH=deploy-config/deployer.json forge script -vvv  --non-interactive --skip-simulation scripts/deploy/Deploy.s.sol:Deploy --rpc-url $L1_RPC --broadcast --private-key $PRIVATE_KEY_DEPLOYER ${process.env.ETHERSCAN_API_KEY ? "--verify ": ""}${process.env.FORGE_FLAGS ?? ""}`,
      { stdio: 'inherit' }
    )
	  console.log("generating STATE_DUMP_PATH")
    execSync(`DEPLOY_CONFIG_PATH=deploy-config/deployer.json DEPLOYMENT_CONTEXT=deployer CONTRACT_ADDRESSES_PATH=deployments/deployer/.deploy STATE_DUMP_PATH=allocs-l2-raw.json forge script -vvv  --non-interactive --skip-simulation scripts/L2Genesis.s.sol:L2Genesis --sig "runWithStateDump()" --private-key $PRIVATE_KEY_DEPLOYER --chain-id $L2_CHAIN_ID`)

    console.log("generating allocs-l2")
    writeFileSync("allocs-l2.json", JSON.stringify({accounts: JSON.parse(readFileSync('allocs-l2-raw.json', 'utf-8'))}, null, 2))

    console.log('generating rollup.json, genesis.json files')
    execSync(`op-node genesis l2 --l1-rpc ${L1_RPC} --l2-allocs allocs-l2-raw.json --deploy-config deploy-config/deployer.json --l1-deployments deployments/deployer/.deploy --outfile.l2 genesis.json --outfile.rollup rollup.json`,
      { stdio: 'inherit' }
    )
    console.log('generating contracts.json file')
    const addrs = JSON.parse(readFileSync(`deployments/deployer/.deploy`, 'utf-8'))
    writeFileSync(
      'contracts.json',
      JSON.stringify(
        {
          AddressManager: addrs.AddressManager,
          L1CrossDomainMessenger: addrs.L1CrossDomainMessengerProxy,
          L1StandardBridge: addrs.L1StandardBridgeProxy,
          OptimismPortal: addrs.OptimismPortalProxy,
          L2OutputOracle: addrs.L2OutputOracleProxy,
          SystemConfig: addrs.SystemConfigProxy,
          SuperchainConfigProxy: addrs.SuperchainConfigProxy,
        },
        null,
        2
      )
    )

    if (process.env.S3_FOLDER) {
      console.log('Uploading to s3 bucket')
      for (const file of [
        'allocs-l2.json',
        'allocs-l2-raw.json',
        'deployments/deployer/.deploy',
        'rollup.json',
        'contracts.json',
        'genesis.json',
        'deploy-config/deployer.json',
      ]) {
        const command = new PutObjectCommand({
          Bucket: S3_BUCKET,
          Body: readFileSync(file),
          Key: S3_PREFIX + file.split('/').slice(-1)[0],
        })
        await s3.send(command)
      }
    }
    if (process.env.REFUND_ADDRESS) {
      console.log('draining deployer wallet to refund address')
      const wallet = new ethers.Wallet(process.env.PRIVATE_KEY_DEPLOYER, provider)
      const balance = await wallet.getBalance();
      console.log(wallet.address, "balance=" + ethers.utils.formatEther(balance))
      const gasPrice = (await provider.getGasPrice()).mul(15).div(10);
      const gasLimit = await wallet.estimateGas({type: 0, to: process.env.REFUND_ADDRESS, gasPrice})
      const gas = gasPrice.mul(gasLimit)
      console.log("gas=" + ethers.utils.formatEther(gas))
      if (balance.lt(gas)) {
        console.log("wallet balance too low")
      } else {
        // no need for EIP-1559 since we're draining the wallet - give extra fees to the miner instead
        const tx = await wallet.sendTransaction({
          type: 0,
          to: process.env.REFUND_ADDRESS,
          value: balance.sub(gas),
          gasPrice,
          gasLimit
        })
        console.log("tx hash", tx.hash)
        const receipt = await tx.wait(1)
        if (receipt.status !== 1)
          console.warn("transaction reverted")
        const remaining = await wallet.getBalance();
        console.log(wallet.address, "remaining=" + ethers.utils.formatEther(remaining));
      }
    }
  }
  console.log('clearing /root/config')
  emptyDirSync('/root/config')
  console.log('copying rollup.json to /root/config')
  copyFileSync('rollup.json', '/root/config/rollup.json')
  console.log('copying contracts.json to /root/config')
  copyFileSync('contracts.json', '/root/config/contracts.json')
  console.log('copying genesis.json to /root/config')
  copyFileSync('genesis.json', '/root/config/genesis.json')
  console.log('creating l2oo-address.txt')
  writeFileSync(
    '/root/config/l2oo-address.txt',
    JSON.parse(readFileSync('contracts.json', 'utf-8')).L2OutputOracle
  )
  console.log('creating jwt token')
  writeFileSync('/root/config/jwt.txt', randomBytes(32).toString('hex'))
}

main()
