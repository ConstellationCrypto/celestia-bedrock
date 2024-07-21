/* eslint-disable prefer-arrow/prefer-arrow-functions */
import { hashWithdrawal } from '@eth-optimism/core-utils'
import { BigNumber, utils, ethers } from 'ethers'

import { LowLevelMessage } from '../interfaces'

const { hexDataLength } = utils

// Constants used by `CrossDomainMessenger.baseGas`
const RELAY_CONSTANT_OVERHEAD = BigNumber.from(200_000)
const RELAY_PER_BYTE_DATA_COST = BigNumber.from(16)
const MIN_GAS_DYNAMIC_OVERHEAD_NUMERATOR = BigNumber.from(64)
const MIN_GAS_DYNAMIC_OVERHEAD_DENOMINATOR = BigNumber.from(63)
const RELAY_CALL_OVERHEAD = BigNumber.from(40_000)
const RELAY_RESERVED_GAS = BigNumber.from(40_000)
const RELAY_GAS_CHECK_BUFFER = BigNumber.from(5_000)

export type ByteArray = Uint8Array
export type Hex = `0x${string}`
export type SliceReturnType<TValue extends ByteArray | Hex> = TValue extends Hex
  ? Hex
  : ByteArray

/**
 * Utility for hashing a LowLevelMessage object.
 *
 * @param message LowLevelMessage object to hash.
 * @returns Hash of the given LowLevelMessage.
 */
export const hashLowLevelMessage = (message: LowLevelMessage): string => {
  return hashWithdrawal(
    message.messageNonce,
    message.sender,
    message.target,
    message.value,
    message.minGasLimit,
    message.message
  )
}

/**
 * Utility for hashing a message hash. This computes the storage slot
 * where the message hash will be stored in state. HashZero is used
 * because the first mapping in the contract is used.
 *
 * @param messageHash Message hash to hash.
 * @returns Hash of the given message hash.
 */
export const hashMessageHash = (messageHash: string): string => {
  const data = ethers.utils.defaultAbiCoder.encode(
    ['bytes32', 'uint256'],
    [messageHash, ethers.constants.HashZero]
  )
  return ethers.utils.keccak256(data)
}

/**
 * Compute the min gas limit for a migrated withdrawal.
 */
export const migratedWithdrawalGasLimit = (
  data: string,
  chainID: number
): BigNumber => {
  // Compute the gas limit and cap at 25 million
  const dataCost = BigNumber.from(hexDataLength(data)).mul(
    RELAY_PER_BYTE_DATA_COST
  )
  let overhead: BigNumber
  if (chainID === 420) {
    overhead = BigNumber.from(200_000)
  } else {
    // Dynamic overhead (EIP-150)
    // We use a constant 1 million gas limit due to the overhead of simulating all migrated withdrawal
    // transactions during the migration. This is a conservative estimate, and if a withdrawal
    // uses more than the minimum gas limit, it will fail and need to be replayed with a higher
    // gas limit.
    const dynamicOverhead = MIN_GAS_DYNAMIC_OVERHEAD_NUMERATOR.mul(
      1_000_000
    ).div(MIN_GAS_DYNAMIC_OVERHEAD_DENOMINATOR)

    // Constant overhead
    overhead = RELAY_CONSTANT_OVERHEAD.add(dynamicOverhead)
      .add(RELAY_CALL_OVERHEAD)
      // Gas reserved for the worst-case cost of 3/5 of the `CALL` opcode's dynamic gas
      // factors. (Conservative)
      // Relay reserved gas (to ensure execution of `relayMessage` completes after the
      // subcontext finishes executing) (Conservative)
      .add(RELAY_RESERVED_GAS)
      // Gas reserved for the execution between the `hasMinGas` check and the `CALL`
      // opcode. (Conservative)
      .add(RELAY_GAS_CHECK_BUFFER)
  }

  let minGasLimit = dataCost.add(overhead)
  if (minGasLimit.gt(25_000_000)) {
    minGasLimit = BigNumber.from(25_000_000)
  }
  return minGasLimit
}

export function opaqueDataToDepositData(opaqueData: Hex): Object {
  let offset = 0
  const mint = slice(opaqueData, offset, offset + 32)
  offset += 32
  const value = slice(opaqueData, offset, offset + 32)
  offset += 32
  const gas = slice(opaqueData, offset, offset + 8)
  offset += 8
  const isCreation = BigInt(slice(opaqueData, offset, offset + 1)) === 1n
  offset += 1
  const data =
    offset > size(opaqueData) - 1
      ? '0x'
      : slice(opaqueData, offset, opaqueData.length)
  return {
    mint: hexToBigInt(mint),
    value: hexToBigInt(value),
    gas: hexToBigInt(gas),
    isCreation,
    data,
  }
}

/**
 * @description Returns a section of the hex or byte array given a start/end bytes offset.
 *
 * @param value The hex or byte array to slice.
 * @param start The start offset (in bytes).
 * @param end The end offset (in bytes).
 */
export function slice<TValue extends ByteArray | Hex>(
  value: TValue,
  start?: number | undefined,
  end?: number | undefined,
  { strict }: { strict?: boolean | undefined } = {}
): SliceReturnType<TValue> {
  if (isHex(value, { strict: false })) {
    return sliceHex(value as Hex, start, end, {
      strict,
    }) as SliceReturnType<TValue>
  }
  return sliceBytes(value as ByteArray, start, end, {
    strict,
  }) as SliceReturnType<TValue>
}

export function size(value: Hex | ByteArray) {
  if (isHex(value, { strict: false })) {
    return Math.ceil((value.length - 2) / 2)
  }
  return value.length
}

export function isHex(
  value: unknown,
  { strict = true }: { strict?: boolean | undefined } = {}
): value is Hex {
  if (!value) {
    return false
  }
  if (typeof value !== 'string') {
    return false
  }
  return strict ? /^0x[0-9a-fA-F]*$/.test(value) : value.startsWith('0x')
}

/**
 * @description Returns a section of the byte array given a start/end bytes offset.
 *
 * @param value The byte array to slice.
 * @param start The start offset (in bytes).
 * @param end The end offset (in bytes).
 */
export function sliceBytes(
  value_: ByteArray,
  start?: number | undefined,
  end?: number | undefined,
  { strict }: { strict?: boolean | undefined } = {}
): ByteArray {
  assertStartOffset(value_, start)
  const value = value_.slice(start, end)
  if (strict) {
    assertEndOffset(value, start, end)
  }
  return value
}

function assertStartOffset(value: Hex | ByteArray, start?: number | undefined) {
  if (typeof start === 'number' && start > 0 && start > size(value) - 1) {
    throw new Object({
      offset: start,
      position: 'start',
      size: size(value),
    })
  }
}

/**
 * @description Returns a section of the hex value given a start/end bytes offset.
 *
 * @param value The hex value to slice.
 * @param start The start offset (in bytes).
 * @param end The end offset (in bytes).
 */
export function sliceHex(
  value_: Hex,
  start?: number | undefined,
  end?: number | undefined,
  { strict }: { strict?: boolean | undefined } = {}
): Hex {
  assertStartOffset(value_, start)
  const value = `0x${value_
    .replace('0x', '')
    .slice((start ?? 0) * 2, (end ?? value_.length) * 2)}` as const
  if (strict) {
    assertEndOffset(value, start, end)
  }
  return value
}

function assertEndOffset(
  value: Hex | ByteArray,
  start?: number | undefined,
  end?: number | undefined
) {
  if (
    typeof start === 'number' &&
    typeof end === 'number' &&
    size(value) !== end - start
  ) {
    throw new Object({
      offset: end,
      position: 'end',
      size: size(value),
    })
  }
}

export function hexToBigInt(hex: Hex): bigint {
  const signed = undefined

  const value = BigInt(hex)
  if (!signed) {
    return value
  }

  const s = (hex.length - 2) / 2
  const max = (1n << (BigInt(s) * 8n - 1n)) - 1n
  if (value <= max) {
    return value
  }

  return value - BigInt(`0x${'f'.padStart(s * 2, 'f')}`) - 1n
}
