// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the GO_LICENSE file.

// -----------------< AnomalRoil 2016
// Please note that for ease of use, this is a very stripped down copy of
// the code from the crypto/RSA package all credit goes to The Go Authors,
// it follows a BSD-style licence that can be found in the GO_LICENSE file
// Further modification were made to use Dudect.

// Its purpose is to try and demonstrate wheither the current crypto/rsa library
//  is vulnerable to timing attack and suffers from timing leaks. And while a timing
//  leak can be found in the leftPad function, it appears that the noise generated by
//  the DecryptOAEP function is too great and so the signal-to-noise ratio too low
//  to exploit the leftPad leak. This leverage a t-test framework called dudect and
//  designed by Oscar Reparaz, Josep Balasch and Ingrid Verbauwhede, all credits due.
// ------------------ AnomalRoil 2016 >

package main

import (
	"crypto/rand"
	"fmt"
	mrand "math/rand"
	"time"
)

// leftPadConst returns a new slice of length size. The contents of input are right
// aligned in the new slice, using the old Copy implementation
func leftPad(input []byte, size int) (out []byte) {
	n := len(input)
	if n > size {
		n = size
	}
	out = make([]byte, size)
	copy(out[size-n:], input)
	return
}

// For the leftPad test:
func prepare_inputs() (input_data [][]byte, classes []int) {
	input_data = make([][]byte, number_measurements)
	classes = make([]int, number_measurements)

	rn := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	for i := 0; i < number_measurements; i++ {
		classes[i] = rn.Intn(2)
		data := make([]byte, 256)
		_, err := rand.Read(data)
		//data, err := hex.DecodeString("73e4952b02c526cccb40bc093f56a9e9065f366e7778de49fadaa91427526377af02f1bb5201e90a9a79bf82a03936f7dce806637b1114d395c14d718d95b909d5292475e79c01b1f7695f0d83ff15a1da819dca0f14e2bb2bb093b24c4364be13f9b65bf2943e1f8f5c2d493f6418e09e645f26c935bd2132ef928179e5e411a26038f78b1defc16b65c96e975cf03ab7e4be3dc0481f2dd4a047ab53f2edaddb13739ad98829bdbc58b520fb227246e5e8e34678d7fe5dcaf0835403e1f0dfb9d49956d9efcfd4afe8e1ba38609557c0e5a8acef75575cc575dc8c053a00e7f22bf077df6ab27a7cb47afd47f6f8ecb14f032ac42d06e705387707817340ba")
		if err != nil {
			fmt.Println("error:", err)
			panic("err")
		}
		if classes[i] == 0 {
			input_data[i] = data
		} else {
			input_data[i] = data[1:]
		}

	}
	return
}

// For the leftPad test:
func do_one_computation(data []byte) {
	size := len(data)
	if len(data) != 256 {
		size = 256
	}
	out := leftPad(data, size)
	out[0] = 0
}
