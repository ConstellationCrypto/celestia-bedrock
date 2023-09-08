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
    const PROPOSER = process.env.PROPOSER_ADDRESS
    const BATCHER = process.env.BATCHER_ADDRESS
    const SEQUENCER = process.env.SEQUENCER_ADDRESS
    const FEE_WALLET = process.env.FEE_WALLET_ADDRESS
    const L1_RPC = process.env.L1_RPC

    const provider = new ethers.providers.JsonRpcProvider(L1_RPC)
    const block = await provider.getBlock('finalized')
    const BLOCKHASH = block.hash
    const TIMESTAMP = block.timestamp

    const json = {
      numDeployConfirmations: Number(process.env.NUM_DEPLOY_CONFIRMATIONS), // 1

      finalSystemOwner: ADMIN,
      portalGuardian: ADMIN,
      controller: DEPLOYER,

      l1StartingBlockTag: BLOCKHASH,

      l1ChainID: Number(process.env.CHAIN_ID), // 5 for goerli
      l2ChainID: Number(process.env.L2_CHAIN_ID), // 42069
      l2BlockTime: Number(process.env.L2_BLOCK_TIME), // 2

      maxSequencerDrift: 600,
      sequencerWindowSize: 21600, // 72 hours
      channelTimeout: 300,

      p2pSequencerAddress: SEQUENCER,
      batchInboxAddress: '0x' + randomBytes(20).toString('hex'), // 0xff00000000000000000000000000000000042069
      batchSenderAddress: BATCHER,

      l2OutputOracleSubmissionInterval: Number(process.env.L2_OUTPUT_ORACLE_SUBMISSION_INTERVAL), // 120
      l2OutputOracleStartingBlockNumber: 0,
      l2OutputOracleStartingTimestamp: TIMESTAMP,

      l2OutputOracleProposer: PROPOSER,
      l2OutputOracleChallenger: ADMIN,

      finalizationPeriodSeconds: Number(
        process.env.FINALIZATION_PERIOD_SECONDS
      ), // 12

      proxyAdminOwner: ADMIN,
      baseFeeVaultRecipient: FEE_WALLET,
      l1FeeVaultRecipient: FEE_WALLET,
      sequencerFeeVaultRecipient: FEE_WALLET,

      baseFeeVaultMinimumWithdrawalAmount: '0x8ac7230489e80000', // 10 ETH
      l1FeeVaultMinimumWithdrawalAmount: '0x8ac7230489e80000', // 10 ETH
      sequencerFeeVaultMinimumWithdrawalAmount: '0x8ac7230489e80000', // 10 ETH
      baseFeeVaultWithdrawalNetwork: 0, // 0 = L1, 1 = L2
      l1FeeVaultWithdrawalNetwork: 0, // 0 = L1, 1 = L2
      sequencerFeeVaultWithdrawalNetwork: 0, // 0 = L1, 1 = L2

      gasPriceOracleOverhead: Number(process.env.GAS_PRICE_ORACLE_OVERHEAD), // 2100
      gasPriceOracleScalar: Number(process.env.GAS_PRICE_ORACLE_SCALAR), // 1000000

      enableGovernance: false, // do not predeploy the governance token onto the l2
      governanceTokenSymbol: 'OP', // unused
      governanceTokenName: 'Optimism', // unused
      governanceTokenOwner: ADMIN, // unused

      l2GenesisBlockGasLimit: '0x1c9c380',
      l2GenesisBlockBaseFeePerGas: '0x3b9aca00', // 1 gwei
      l2GenesisRegolithTimeOffset: '0x0',

      eip1559Denominator: Number(process.env.EIP1559Denominator), // 50
      eip1559Elasticity: Number(process.env.EIP1559Elasticity), // 10
    }

    writeFileSync('deploy-config/deployer.json', JSON.stringify(json, null, 2))
    // TODO: set up automatic contract verification with etherscan
    // "--verify --verifier sourcify" works for sourcify.
    const FORGE_CMD =
      'DEPLOYMENT_CONTEXT=deployer forge script -vvv scripts/Deploy.s.sol:Deploy --rpc-url $L1_RPC'
    execSync(
      `${FORGE_CMD} --broadcast --private-key $PRIVATE_KEY_DEPLOYER && ${FORGE_CMD} --sig 'sync()'`,
      { stdio: 'inherit' }
    )
    console.log('generating rollup.json, genesis.json files')
    execSync(
      `op-node genesis l2 \
        --deploy-config deploy-config/deployer.json \
        --deployment-dir deployments/deployer/ \
        --outfile.l2 genesis.json \
        --outfile.rollup rollup.json \
        --l1-rpc ${L1_RPC}`,
      { stdio: 'inherit' }
    )
    console.log('generating contracts.json file')
    const getAddress = (x) =>
      JSON.parse(readFileSync(`deployments/deployer/${x}.json`, 'utf-8'))
        .address
    writeFileSync(
      'contracts.json',
      JSON.stringify(
        {
          AddressManager: getAddress('AddressManager'),
          L1CrossDomainMessenger: getAddress('L1CrossDomainMessengerProxy'),
          L1StandardBridge: getAddress('L1StandardBridgeProxy'),
          OptimismPortal: getAddress('OptimismPortalProxy'),
          L2OutputOracle: getAddress('L2OutputOracleProxy'),
        },
        null,
        2
      )
    )

    if (process.env.S3_FOLDER) {
      console.log('Uploading to s3 bucket')
      for (const file of [
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
  }
  console.log('clearing /root/config')
  emptyDirSync('/root/config')
  console.log('clearing /root/datadir')
  emptyDirSync('/root/datadir')
  console.log('clearing /root/datadir2')
  emptyDirSync('/root/datadir2')
  console.log('copying rollup.json to /root/config')
  copyFileSync('rollup.json', '/root/config/rollup.json')
  console.log('copying constracts.json to /root/config')
  copyFileSync('contracts.json', '/root/config/contracts.json')
  console.log('creating l2oo-address.txt')
  writeFileSync(
    '/root/config/l2oo-address.txt',
    JSON.parse(readFileSync('contracts.json', 'utf-8')).L2OutputOracle
  )
  console.log('creating jwt token')
  writeFileSync('/root/config/jwt.txt', randomBytes(32).toString('hex'))
  console.log('initializing geth datadir')
  writeFileSync('/root/datadir/password', 'pwd')
  writeFileSync('block-signer-key', process.env.SEQUENCER_PRIVATE_KEY)
  execSync(
    'geth account import --datadir=/root/datadir --password=/root/datadir/password block-signer-key',
    { stdio: 'inherit' }
  )
  execSync('geth init --datadir=/root/datadir genesis.json', {
    stdio: 'inherit',
  })
  console.log('copy datadir to datadir2')
  copySync('/root/datadir', '/root/datadir2')
  console.log('Done')
}

main()
