import { generateMnemonic, mnemonicToSeedSync } from "@scure/bip39";
import { wordlist } from "@scure/bip39/wordlists/english";
import { HDKey } from "@scure/bip32";
import { sha256 } from "@noble/hashes/sha256";
import { ripemd160 } from "@noble/hashes/ripemd160";
import { bech32 } from "@scure/base";

const mnemonic = generateMnemonic(wordlist, 256);
const seed = mnemonicToSeedSync(mnemonic, "");

const root = HDKey.fromMasterSeed(seed);
const node = root.derive("m/44'/118'/0'/0");
const xpub = node.publicExtendedKey;

const pubkey = node.publicKey;
const addrBytes = ripemd160(sha256(pubkey));
const address = bech32.encode("dora", bech32.toWords(addrBytes));

console.log("Mnemonic:");
console.log(mnemonic);
console.log("\nXPub (m/44'/118'/0'/0):");
console.log(xpub);
console.log("\nSample address (index 0):");
console.log(address);
console.log("\nNext step:");
console.log("- Store mnemonic offline");
console.log("- Put xpub into configs/config.yaml (wallet.xpub)");
