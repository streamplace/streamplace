import {
  createStreamKey,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import { useEffect, useState } from "react";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { View, Text, Button } from "tamagui";
import { Secp256k1Keypair, bytesToMultibase } from "@atproto/crypto";
import { privateKeyToAccount } from "viem/accounts";

export default function StreamKeyScreen() {
  const userProfile = useAppSelector(selectUserProfile);
  const [privateKey, setPrivateKey] = useState<string | null>(null);
  const [publicKey, setPublicKey] = useState<string | null>(null);
  const [address, setAddress] = useState<string | null>(null);
  const [did, setDid] = useState<string | null>(null);
  const dispatch = useAppDispatch();
  useEffect(() => {
    // const privateKey = generatePrivateKey();
    // setPrivateKey(privateKey);
    // const account = privateKeyToAccount(privateKey);

    // setAddress(account.address.toLowerCase() as `0x${string}`);
    // setPublicKey(account.publicKey);
    // const publicKeyBytes = new Uint8Array(
    //   account.publicKey
    //     .slice(2)
    //     .match(/.{1,2}/g)
    //     ?.map((byte) => parseInt(byte, 16)) || [],
    // );
    // setPublicKey(bytesToMultibase(publicKeyBytes, "base58btc"));

    async function createKey() {
      if (!userProfile) return;
      const keypair = await Secp256k1Keypair.create({ exportable: true });
      setDid(keypair.did());
      const exportedKey = await keypair.export();
      const didBytes = new TextEncoder().encode(userProfile.did);
      const combinedKey = new Uint8Array([...exportedKey, ...didBytes]);
      const multibaseKey = bytesToMultibase(combinedKey, "base58btc");
      const hexKey = Array.from(exportedKey)
        .map((b) => b.toString(16).padStart(2, "0"))
        .join("");
      const account = await privateKeyToAccount(`0x${hexKey}`);
      setPrivateKey(multibaseKey);
      setPublicKey(account.publicKey);
      setAddress(account.address.toLowerCase() as `0x${string}`);
    }
    createKey();
  }, [userProfile]);
  return (
    <View f={1} jc="center" ai="center">
      <View maxWidth={600}>
        <Text wordWrap="break-word">Private Key: {privateKey}</Text>
        <Text wordWrap="break-word">Public Key: {publicKey}</Text>
        <Text wordWrap="break-word">Address: {address}</Text>
        <Text wordWrap="break-word">DID: {did}</Text>
        <Button
          onPress={() => {
            if (!did) return;
            dispatch(createStreamKey({ signingKey: did }));
          }}
        >
          Create Stream Key
        </Button>
      </View>
    </View>
  );
}
