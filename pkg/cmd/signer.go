package cmd

import (
	"context"
	"crypto"
	"fmt"
	"os"
	"strconv"

	"github.com/ThalesGroup/crypto11"
	"golang.org/x/term"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	v0 "stream.place/streamplace/pkg/schema/v0"

	"stream.place/streamplace/pkg/crypto/signers/eip712"
	"stream.place/streamplace/pkg/log"
)

func createSigner(ctx context.Context, cli *config.CLI) (crypto.Signer, error) {
	schema, err := v0.MakeV0Schema()
	if err != nil {
		return nil, err
	}
	eip712signer, err := eip712.MakeEIP712Signer(ctx, &eip712.EIP712SignerOptions{
		Schema:              schema,
		EthKeystorePath:     cli.EthKeystorePath,
		EthAccountAddr:      cli.EthAccountAddr,
		EthKeystorePassword: cli.EthPassword,
	})
	if err != nil {
		return nil, err
	}
	var signer crypto.Signer = eip712signer
	if cli.PKCS11ModulePath != "" {
		conf := &crypto11.Config{
			Path: cli.PKCS11ModulePath,
		}
		count := 0
		for _, val := range []string{cli.PKCS11TokenSlot, cli.PKCS11TokenLabel, cli.PKCS11TokenSerial} {
			if val != "" {
				count += 1
			}
		}
		if count != 1 {
			return nil, fmt.Errorf("need exactly one of pkcs11-token-slot, pkcs11-token-label, or pkcs11-token-serial (got %d)", count)
		}
		if cli.PKCS11TokenSlot != "" {
			num, err := strconv.ParseInt(cli.PKCS11TokenSlot, 10, 16)
			if err != nil {
				return nil, fmt.Errorf("error parsing pkcs11-slot: %w", err)
			}
			numint := int(num)
			// why does crypto11 want this as a reference? odd.
			conf.SlotNumber = &numint
		}
		if cli.PKCS11TokenLabel != "" {
			conf.TokenLabel = cli.PKCS11TokenLabel
		}
		if cli.PKCS11TokenSerial != "" {
			conf.TokenSerial = cli.PKCS11TokenSerial
		}
		pin := cli.PKCS11Pin
		if pin == "" {
			fmt.Printf("Please enter PKCS11 PIN: ")
			password, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println("")
			if err != nil {
				return nil, fmt.Errorf("error reading PKCS11 password: %w", err)
			}
			pin = string(password)
		}
		conf.Pin = pin

		sc, err := crypto11.Configure(conf)
		if err != nil {
			return nil, fmt.Errorf("error initalizing PKCS11 HSM: %w", err)
		}
		var id []byte = nil
		var label []byte = nil
		if cli.PKCS11KeypairID != "" {
			num, err := strconv.ParseInt(cli.PKCS11KeypairID, 10, 8)
			if err != nil {
				return nil, fmt.Errorf("error parsing pkcs11-keypair-id: %w", err)
			}
			id = []byte{byte(num)}
		}
		if cli.PKCS11KeypairLabel != "" {
			label = []byte(cli.PKCS11KeypairLabel)
		}
		hwsigner, err := sc.FindKeyPair(id, label)
		if err != nil {
			return nil, fmt.Errorf("error finding keypair on PKCS11 token: %w", err)
		}
		if hwsigner == nil {
			return nil, fmt.Errorf("keypair on token not found (tried id='%s' label='%s')", cli.PKCS11KeypairID, cli.PKCS11KeypairLabel)
		}
		addr, err := signers.HexAddrFromSigner(hwsigner)
		if err != nil {
			return nil, fmt.Errorf("error getting ethereum address for hardware keypair: %w", err)
		}
		log.Log(ctx, "successfully initialized hardware signer", "address", addr)
		signer = hwsigner
	}
	return signer, nil
}
