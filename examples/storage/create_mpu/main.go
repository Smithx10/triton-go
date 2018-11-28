//
// Copyright (c) 2018, Joyent, Inc. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
//

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"encoding/pem"

	triton "github.com/joyent/triton-go"
	"github.com/joyent/triton-go/authentication"
	"github.com/joyent/triton-go/storage"
)

func main() {
	keyID := os.Getenv("TRITON_KEY_ID")
	accountName := os.Getenv("TRITON_ACCOUNT")
	keyMaterial := os.Getenv("TRITON_KEY_MATERIAL")
	userName := os.Getenv("TRITON_USER")

	var signer authentication.Signer
	var err error

	if keyMaterial == "" {
		input := authentication.SSHAgentSignerInput{
			KeyID:       keyID,
			AccountName: accountName,
			Username:    userName,
		}
		signer, err = authentication.NewSSHAgentSigner(input)
		if err != nil {
			log.Fatalf("Error Creating SSH Agent Signer: %v", err)
		}
	} else {
		var keyBytes []byte
		if _, err = os.Stat(keyMaterial); err == nil {
			keyBytes, err = ioutil.ReadFile(keyMaterial)
			if err != nil {
				log.Fatalf("Error reading key material from %s: %s",
					keyMaterial, err)
			}
			block, _ := pem.Decode(keyBytes)
			if block == nil {
				log.Fatalf(
					"Failed to read key material '%s': no key found", keyMaterial)
			}

			if block.Headers["Proc-Type"] == "4,ENCRYPTED" {
				log.Fatalf(
					"Failed to read key '%s': password protected keys are\n"+
						"not currently supported. Please decrypt the key prior to use.", keyMaterial)
			}

		} else {
			keyBytes = []byte(keyMaterial)
		}

		input := authentication.PrivateKeySignerInput{
			KeyID:              keyID,
			PrivateKeyMaterial: keyBytes,
			AccountName:        accountName,
			Username:           userName,
		}
		signer, err = authentication.NewPrivateKeySigner(input)
		if err != nil {
			log.Fatalf("Error Creating SSH Private Key Signer: %v", err)
		}
	}

	config := &triton.ClientConfig{
		MantaURL:    os.Getenv("MANTA_URL"),
		AccountName: accountName,
		Username:    userName,
		Signers:     []authentication.Signer{signer},
	}

	client, err := storage.NewClient(config)
	if err != nil {
		log.Fatalf("NewClient: %v", err)
	}

	mpuBody := storage.CreateMpuBody{
		ObjectPath: "/" + accountName + "/stor/foo2.txt",
	}

	createMpuInput := &storage.CreateMpuInput{
		DurabilityLevel: 2,
		Body:            mpuBody,
	}

	response := &storage.CreateMpuOutput{}
	response, err = client.Objects().CreateMultipartUpload(context.Background(), createMpuInput)
	if err != nil {
		log.Fatalf("storage.Objects.CreateMpu: %v", err)
	}
	fmt.Printf("Response Body\nid: %s\npartsDirectory: %s\n", response.Id, response.PartsDirectory)
	fmt.Println("Successfully created MPU for /tmp/foo.txt!")

	reader, err := os.Open("/tmp/foo.txt")
	if err != nil {
		log.Fatalf("os.Open: %v", err)
	}
	defer reader.Close()

	uploadPartInput := &storage.UploadPartInput{
		ObjectDirectoryPath: response.PartsDirectory,
		PartNum:	     0,
		ObjectReader:	     reader,
	}

	response2 := &storage.UploadPartOutput{}
	response2, err = client.Objects().UploadPart(context.Background(), uploadPartInput)
	if err != nil {
		log.Fatalf("storage.Objects.UploadPart: %v", err)
	}
	fmt.Println("Successfully uploaded /tmp/foo.txt part 0!")

	var parts []string
	fmt.Printf("Part: %s\n", response2.Part)
	parts = append(parts, response2.Part)
	commitBody := storage.CommitMpuBody{
		Parts: parts,
	}

	commitMpuInput := &storage.CommitMpuInput{
		ObjectDirectoryPath: response.PartsDirectory,
		Body: commitBody,
	}

	err = client.Objects().CommitMultipartUpload(context.Background(), commitMpuInput)
	if err != nil {
		log.Fatalf("storage.Objects.CommitMultipartUpload: %v", err)
	}
	fmt.Println("Successfully committed /tmp/foo.txt!")


}
