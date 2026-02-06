package chain

import (
	"crypto/sha256"
	"errors"

	"github.com/btcsuite/btcd/btcutil/bech32"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"golang.org/x/crypto/ripemd160"
)

type AddressDeriver struct {
	XPub   string
	Prefix string
}

// Derive expects XPub at path m/44'/118'/0'/0 and derives child index i.

func (d AddressDeriver) Derive(index uint32) (string, error) {
	if d.XPub == "" {
		return "", errors.New("xpub is not configured")
	}
	if d.Prefix == "" {
		return "", errors.New("bech32 prefix is not configured")
	}

	key, err := hdkeychain.NewKeyFromString(d.XPub)
	if err != nil {
		return "", err
	}
	child, err := key.Derive(index)
	if err != nil {
		return "", err
	}

	pubKey, err := child.ECPubKey()
	if err != nil {
		return "", err
	}

	compressed := pubKey.SerializeCompressed()
	hash := sha256.Sum256(compressed)
	rip := ripemd160.New()
	_, _ = rip.Write(hash[:])
	addr := rip.Sum(nil)

	converted, err := bech32.ConvertBits(addr, 8, 5, true)
	if err != nil {
		return "", err
	}
	return bech32.Encode(d.Prefix, converted)
}
