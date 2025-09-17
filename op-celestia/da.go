package celestia

// DerivationVersionCelestia is a byte marker for celestia references submitted
// to the batch inbox address as calldata.
// Mnemonic 0xce = celestia
// version 0xce references are encoded as:
// [8]byte block height ++ [32]byte commitment
// in little-endian encoding.
// see: https://github.com/rollkit/celestia-da/blob/1f2df375fd2fcc59e425a50f7eb950daa5382ef0/celestia.go#L141-L160
const DerivationVersionCelestia = 0xce

// SplitID splits a Celestia ID into height and commitment.
// The ID format is: [8]byte block height ++ [32]byte commitment (little-endian)
func SplitID(id []byte) (uint64, []byte) {
	if len(id) < 40 { // 8 bytes height + 32 bytes commitment
		return 0, nil
	}
	// Extract height (first 8 bytes, little-endian)
	height := uint64(id[0]) | uint64(id[1])<<8 | uint64(id[2])<<16 | uint64(id[3])<<24 |
		uint64(id[4])<<32 | uint64(id[5])<<40 | uint64(id[6])<<48 | uint64(id[7])<<56
	// Extract commitment (remaining 32 bytes)
	commitment := make([]byte, 32)
	copy(commitment, id[8:40])
	return height, commitment
}
