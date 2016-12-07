// This is a modified copy of the code from the crypto/RSA package
// all credits goes to The Go Authors, it follows a BSD-style licence
// that can be found in the Go LICENSE file

// Its purpose is to demonstrate that the current crypto/rsa library is
// vulnerable to Manger attacks, cf. J. Manger. A Chosen Ciphertext Attack on RSA Optimal
// Asymmetric Encryption Padding (OAEP) as Standardized in PKCS #1
// v2.0. In J. Kilian, editor, Advances in Cryptology.

package main

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"hash"
	"io"
	"log"
	"math/big"
	mrand "math/rand"
	"os"
	"time"
)

// Please note that for ease of use, we are exposing the oracle at first
// and then later trying to use a timing oracle
// RSA library, a bit stripped:

var bigZero = big.NewInt(0)
var bigOne = big.NewInt(1)

var numberOfZeros int

// A PublicKey represents the public part of an RSA key.
type PublicKey struct {
	N *big.Int // modulus
	E int      // public exponent

}

// OAEPOptions is an interface for passing options to OAEP decryption using the
// crypto.Decrypter interface.
type OAEPOptions struct {
	// Hash is the hash function that will be used when generating the mask.
	Hash crypto.Hash
	// Label is an arbitrary byte string that must be equal to the value
	// used when encrypting.
	Label []byte
}

var (
	errPublicModulus       = errors.New("crypto/rsa: missing public modulus")
	errPublicExponentSmall = errors.New("crypto/rsa: public exponent too small")
	errPublicExponentLarge = errors.New("crypto/rsa: public exponent too large")
)

// checkPub sanity checks the public key before we use it.
// We require pub.E to fit into a 32-bit integer so that we
// do not have different behavior depending on whether
// int is 32 or 64 bits. See also
// http://www.imperialviolet.org/2012/03/16/rsae.html.
func checkPub(pub *PublicKey) error {
	if pub.N == nil {
		return errPublicModulus

	}
	if pub.E < 2 {
		return errPublicExponentSmall

	}
	if pub.E > 1<<31-1 {
		return errPublicExponentLarge

	}
	return nil

}

// A PrivateKey represents an RSA key
type PrivateKey struct {
	PublicKey            // public part.
	D         *big.Int   // private exponent
	Primes    []*big.Int // prime factors of N, has >= 2 elements.

	// Precomputed contains precomputed values that speed up private
	// operations, if available.
	Precomputed PrecomputedValues
}

// The private key used
var test2048Key = &PrivateKey{
	PublicKey: PublicKey{
		N: fromBase16("b3a6b8dac202f283b94ed148cf5eedd6a9990ee2cc42e9955c5b06ec40c23a205de3c0ed7f0fbc29b3d38cdffc9129f2e8b2f54a0df471e7f27c0f2eac1298b68a802ae1f2dccf2ebae134b4cbc3866b3b1e65b44ab541b80609a62c09322e46e5e1ff3e05eb2af7ca5f4df2c62f3107d4647bee1a77d3f5c787c583ee834b25bbb0fcbb4ed9e97cef8e8f2b8f947ebdefda9c1e0af23ac7b2445ba3b3d483a76f007fca88cd1f13b2f85b1d435c3000bd1d6fa245489c6239e8b1b6648dcbcb2463589f76df043188e84cb458858ed1f1de3ae89025111854602d9bd6cc6da5369e9c7c32430d25129f23ce37d281883f4de1bd5787d52815c13c2009829fdd"),
		E: 17,
	},
	D: fromBase16("3f680501ea1f286ab9df9528c1a908a61dbd8cc88453d9f87af2f36271357ded4e506235b45fe80eb7f04fd6956069288e5d47838c74646ffb3ad82e97159f4f7c2d3c4fbf20c19805b8e56cfc9f5c9e5119c98aed30ea04b6d63aa6215d0146330478340216c3defc21a30a6410a7e4a550a435eb3959de466c2797f9d3fc671416ef3451c534b74e2793f123adba26ebfd2daca1ea8a7d60737528b85dcbae2c310adc1d9d931e4c016fe24a938f13c5b98226ce19320866c73b006d07f0066a5d009f83be55fd8f994fcdc08679ae13ff7a7b581de3523cee82f3955087f2cbcd648087839ebbb2876498fbb9ed78e69bf0903794a30a77409c614f2d9719"),
	Primes: []*big.Int{
		fromBase16("e15e6eb27860142b2b68b68b4260dc9c595fdc9dfa5eb2f9ed4a530c70ffbb1a6c201ff4a292d58134a5ebd52776806ece5168d1e7becdf20ddb4212dc57e1994f197b858e5202163831bbbbeef99a9a0c3a0aef8080582a7a4e188bbd27780d6cd6e1c65753b54a0969589f35c494e5654d75c0be6c46f04070d9a3fa69b94b"),
		fromBase16("cc1192f49975bff51160601fbd7212b34f2d68c19b25aa1533b2e74e8dcb0774db0016663cfbd36751a3b246f3439a2f3e93c0b7c0426b585e2e4877a89f6cca5297b0ab489c63cce4842edc1d644620025054f0eb500a2f82c3a2089d40c9bd3301c89f05a5161c8f60d8d2e37f2121a1f14263fba1159a2e1952130417ba77"),
	},
}

// Public returns the public key corresponding to priv.
func (priv *PrivateKey) Public() crypto.PublicKey {
	return &priv.PublicKey

}

type PrecomputedValues struct {
	Dp, Dq *big.Int // D mod (P-1) (or mod Q-1)
	Qinv   *big.Int // Q^-1 mod P

	// CRTValues is used for the 3rd and subsequent primes. Due to a
	// historical accident, the CRT for the first two primes is handled
	// differently in PKCS#1 and interoperability is sufficiently
	// important that we mirror this.
	CRTValues []CRTValue
}

// CRTValue contains the precomputed Chinese remainder theorem values.
type CRTValue struct {
	Exp   *big.Int // D mod (prime-1).
	Coeff *big.Int // R·Coeff ≡ 1 mod Prime.
	R     *big.Int // product of primes prior to this (inc p and q).

}

// Validate performs basic sanity checks on the key.
// It returns nil if the key is valid, or else an error describing a problem.
func (priv *PrivateKey) Validate() error {
	if err := checkPub(&priv.PublicKey); err != nil {
		return err

	}

	// Check that Πprimes == n.
	modulus := new(big.Int).Set(bigOne)
	for _, prime := range priv.Primes {
		// Any primes ≤ 1 will cause divide-by-zero panics later.
		if prime.Cmp(bigOne) <= 0 {
			return errors.New("crypto/rsa: invalid prime value")

		}
		modulus.Mul(modulus, prime)

	}
	if modulus.Cmp(priv.N) != 0 {
		return errors.New("crypto/rsa: invalid modulus")

	}

	// Check that de ≡ 1 mod p-1, for each prime.
	// This implies that e is coprime to each p-1 as e has a multiplicative
	// inverse. Therefore e is coprime to lcm(p-1,q-1,r-1,...) =
	// exponent(ℤ/nℤ). It also implies that a^de ≡ a mod p as a^(p-1) ≡ 1
	// mod p. Thus a^de ≡ a mod n for all a coprime to n, as required.
	congruence := new(big.Int)
	de := new(big.Int).SetInt64(int64(priv.E))
	de.Mul(de, priv.D)
	for _, prime := range priv.Primes {
		pminus1 := new(big.Int).Sub(prime, bigOne)
		congruence.Mod(de, pminus1)
		if congruence.Cmp(bigOne) != 0 {
			return errors.New("crypto/rsa: invalid exponents")

		}

	}
	return nil

}

// incCounter increments a four byte, big-endian counter.
func incCounter(c *[4]byte) {
	if c[3]++; c[3] != 0 {
		return

	}
	if c[2]++; c[2] != 0 {
		return

	}
	if c[1]++; c[1] != 0 {
		return

	}
	c[0]++

}

// mgf1XOR XORs the bytes in out with a mask generated using the MGF1 function
// specified in PKCS#1 v2.1.
func mgf1XOR(out []byte, hash hash.Hash, seed []byte) {
	var counter [4]byte
	var digest []byte

	done := 0
	for done < len(out) {
		hash.Write(seed)
		hash.Write(counter[0:4])
		digest = hash.Sum(digest[:0])
		hash.Reset()

		for i := 0; i < len(digest) && done < len(out); i++ {
			out[done] ^= digest[i]
			done++

		}
		incCounter(&counter)

	}

}

// ErrMessageTooLong is returned when attempting to encrypt a message which is
// too large for the size of the public key.
var ErrMessageTooLong = errors.New("crypto/rsa: message too long for RSA public key size")

func encrypt(c *big.Int, pub *PublicKey, m *big.Int) *big.Int {
	e := big.NewInt(int64(pub.E))
	c.Exp(m, e, pub.N)
	return c

}

// EncryptOAEP encrypts the given message with RSA-OAEP.
//
// OAEP is parameterised by a hash function that is used as a random oracle.
// Encryption and decryption of a given message must use the same hash function
// and sha256.New() is a reasonable choice.
//
// The random parameter is used as a source of entropy to ensure that
// encrypting the same message twice doesn't result in the same ciphertext.
//
// The label parameter may contain arbitrary data that will not be encrypted,
// but which gives important context to the message. For example, if a given
// public key is used to decrypt two types of messages then distinct label
// values could be used to ensure that a ciphertext for one purpose cannot be
// used for another by an attacker. If not required it can be empty.
//
// The message must be no longer than the length of the public modulus minus
// twice the hash length, minus a further 2.
func EncryptOAEP(hash hash.Hash, random io.Reader, pub *PublicKey, msg []byte, label []byte) ([]byte, error) {
	if err := checkPub(pub); err != nil {
		return nil, err
	}
	hash.Reset()
	k := (pub.N.BitLen() + 7) / 8
	if len(msg) > k-2*hash.Size()-2 {
		return nil, ErrMessageTooLong
	}

	hash.Write(label)
	lHash := hash.Sum(nil)
	hash.Reset()

	em := make([]byte, k)
	seed := em[1 : 1+hash.Size()]
	db := em[1+hash.Size():]

	copy(db[0:hash.Size()], lHash)
	db[len(db)-len(msg)-1] = 1
	copy(db[len(db)-len(msg):], msg)

	_, err := io.ReadFull(random, seed)
	if err != nil {
		return nil, err
	}

	mgf1XOR(db, hash, seed)
	mgf1XOR(seed, hash, db)

	m := new(big.Int)
	m.SetBytes(em)
	c := encrypt(new(big.Int), pub, m)
	out := c.Bytes()

	if len(out) < k {
		// If the output is too small, we need to left-pad with zeros.
		t := make([]byte, k)
		copy(t[k-len(out):], out)
		out = t
	}

	return out, nil
}

// ErrDecryption represents a failure to decrypt a message.
// It is deliberately vague to avoid adaptive attacks.
var ErrDecryption = errors.New("crypto/rsa: decryption error")

// ErrVerification represents a failure to verify a signature.
// It is deliberately vague to avoid adaptive attacks.
var ErrVerification = errors.New("crypto/rsa: verification error")

// modInverse returns ia, the inverse of a in the multiplicative group of prime
// order n. It requires that a be a member of the group (i.e. less than n).
func modInverse(a, n *big.Int) (ia *big.Int, ok bool) {
	g := new(big.Int)
	x := new(big.Int)
	y := new(big.Int)
	g.GCD(x, y, a, n)
	if g.Cmp(bigOne) != 0 {
		// In this case, a and n aren't coprime and we cannot calculate
		// the inverse. This happens because the values of n are nearly
		// prime (being the product of two primes) rather than truly
		// prime.
		return
	}

	if x.Cmp(bigOne) < 0 {
		// 0 is not the multiplicative inverse of any element so, if x
		// < 1, then x is negative.
		x.Add(x, n)
	}

	return x, true
}

// Precompute performs some calculations that speed up private key operations
// in the future.
func (priv *PrivateKey) Precompute() {
	if priv.Precomputed.Dp != nil {
		return
	}

	priv.Precomputed.Dp = new(big.Int).Sub(priv.Primes[0], bigOne)
	priv.Precomputed.Dp.Mod(priv.D, priv.Precomputed.Dp)

	priv.Precomputed.Dq = new(big.Int).Sub(priv.Primes[1], bigOne)
	priv.Precomputed.Dq.Mod(priv.D, priv.Precomputed.Dq)

	priv.Precomputed.Qinv = new(big.Int).ModInverse(priv.Primes[1], priv.Primes[0])

	r := new(big.Int).Mul(priv.Primes[0], priv.Primes[1])
	priv.Precomputed.CRTValues = make([]CRTValue, len(priv.Primes)-2)
	for i := 2; i < len(priv.Primes); i++ {
		prime := priv.Primes[i]
		values := &priv.Precomputed.CRTValues[i-2]

		values.Exp = new(big.Int).Sub(prime, bigOne)
		values.Exp.Mod(priv.D, values.Exp)

		values.R = new(big.Int).Set(r)
		values.Coeff = new(big.Int).ModInverse(r, prime)

		r.Mul(r, prime)
	}
}

// decrypt performs an RSA decryption, resulting in a plaintext integer. If a
// random source is given, RSA blinding is used.
func decrypt(random io.Reader, priv *PrivateKey, c *big.Int) (m *big.Int, err error) {
	// TODO(agl): can we get away with reusing blinds?
	if c.Cmp(priv.N) > 0 {
		//fmt.Println("cipher bigger than N")
		err = fmt.Errorf("cbiggern")
		return
	}
	if priv.N.Sign() == 0 {
		return nil, ErrDecryption
	}

	var ir *big.Int
	if random != nil {
		// Blinding enabled. Blinding involves multiplying c by r^e.
		// Then the decryption operation performs (m^e * r^e)^d mod n
		// which equals mr mod n. The factor of r can then be removed
		// by multiplying by the multiplicative inverse of r.

		var r *big.Int

		for {
			r, err = rand.Int(random, priv.N)
			if err != nil {
				return

			}
			if r.Cmp(bigZero) == 0 {
				r = bigOne

			}
			var ok bool
			ir, ok = modInverse(r, priv.N)
			if ok {
				break

			}

		}
		bigE := big.NewInt(int64(priv.E))
		rpowe := new(big.Int).Exp(r, bigE, priv.N) // N != 0
		cCopy := new(big.Int).Set(c)
		cCopy.Mul(cCopy, rpowe)
		cCopy.Mod(cCopy, priv.N)
		c = cCopy

	}

	if priv.Precomputed.Dp == nil {
		m = new(big.Int).Exp(c, priv.D, priv.N)

	} else {
		// We have the precalculated values needed for the CRT.
		m = new(big.Int).Exp(c, priv.Precomputed.Dp, priv.Primes[0])
		m2 := new(big.Int).Exp(c, priv.Precomputed.Dq, priv.Primes[1])
		m.Sub(m, m2)
		if m.Sign() < 0 {
			m.Add(m, priv.Primes[0])

		}
		m.Mul(m, priv.Precomputed.Qinv)
		m.Mod(m, priv.Primes[0])
		m.Mul(m, priv.Primes[1])
		m.Add(m, m2)

		for i, values := range priv.Precomputed.CRTValues {
			prime := priv.Primes[2+i]
			m2.Exp(c, values.Exp, prime)
			m2.Sub(m2, m)
			m2.Mul(m2, values.Coeff)
			m2.Mod(m2, prime)
			if m2.Sign() < 0 {
				m2.Add(m2, prime)
			}
			m2.Mul(m2, values.R)
			m.Add(m, m2)
		}
	}

	if ir != nil {
		// Unblind.
		m.Mul(m, ir)
		m.Mod(m, priv.N)
	}

	return
}

// DecryptOAEP decrypts ciphertext using RSA-OAEP.

// OAEP is parameterised by a hash function that is used as a random oracle.
// Encryption and decryption of a given message must use the same hash function
// and sha256.New() is a reasonable choice.
//
// The random parameter, if not nil, is used to blind the private-key operation
// and avoid timing side-channel attacks. Blinding is purely internal to this
// function – the random data need not match that used when encrypting.
//
// The label parameter must match the value given when encrypting. See
// EncryptOAEP for details.
func DecryptOAEP(hash hash.Hash, random io.Reader, priv *PrivateKey, ciphertext []byte, label []byte) ([]byte, error) {
	if err := checkPub(&priv.PublicKey); err != nil {
		return nil, err
	}
	k := (priv.N.BitLen() + 7) / 8
	if len(ciphertext) > k ||
		k < hash.Size()*2+2 {
		fmt.Println("too long, k=", k, "and cipher=", len(ciphertext))
		return nil, ErrDecryption
	}

	c := new(big.Int).SetBytes(ciphertext)

	m, err := decrypt(random, priv, c)
	if err != nil {
		return nil, err
	}

	hash.Write(label)
	lHash := hash.Sum(nil)
	hash.Reset()

	// Converting the plaintext number to bytes will strip any
	// leading zeros so we may have to left pad. We do this unconditionally
	// to avoid leaking timing information. (Although we still probably
	// leak the number of leading zeros. It's not clear that we can do
	// anything about this.)
	em := leftPad(m.Bytes(), k)

	firstByteIsZero := subtle.ConstantTimeByteEq(em[0], 0)

	seed := em[1 : hash.Size()+1]
	db := em[hash.Size()+1:]

	mgf1XOR(seed, hash, db)
	mgf1XOR(db, hash, seed)

	lHash2 := db[0:hash.Size()]

	// We have to validate the plaintext in constant time in order to avoid
	// attacks like: J. Manger. A Chosen Ciphertext Attack on RSA Optimal
	// Asymmetric Encryption Padding (OAEP) as Standardized in PKCS #1
	// v2.0. In J. Kilian, editor, Advances in Cryptology.
	lHash2Good := subtle.ConstantTimeCompare(lHash, lHash2)

	// The remainder of the plaintext must be zero or more 0x00, followed
	// by 0x01, followed by the message.
	//   lookingForIndex: 1 iff we are still looking for the 0x01
	//   index: the offset of the first 0x01 byte
	//   invalid: 1 iff we saw a non-zero byte before the 0x01.
	var lookingForIndex, index, invalid int
	lookingForIndex = 1
	rest := db[hash.Size():]

	for i := 0; i < len(rest); i++ {
		equals0 := subtle.ConstantTimeByteEq(rest[i], 0)
		equals1 := subtle.ConstantTimeByteEq(rest[i], 1)
		index = subtle.ConstantTimeSelect(lookingForIndex&equals1, i, index)
		lookingForIndex = subtle.ConstantTimeSelect(equals1, 0, lookingForIndex)
		invalid = subtle.ConstantTimeSelect(lookingForIndex&^equals0, 1, invalid)
	}

	if firstByteIsZero&lHash2Good&^invalid&^lookingForIndex != 1 {
		return nil, ErrDecryption
	}

	return rest[index+1:], nil
}

// leftPad returns a new slice of length size. The contents of input are right
// aligned in the new slice.
func leftPad(input []byte, size int) (out []byte) {
	n := len(input)
	if n > size {
		n = size
	}
	out = make([]byte, size)

	numberOfZeros = len(out) - n
	copy(out[len(out)-n:], input)
	return
}

// leftPadConst returns a new slice of length size. The contents of input are right
// aligned in the new slice, using the old Copy implementation
func leftPadConst(input []byte, size int) (out []byte) {
	n := len(input)
	if n > size {
		n = size
	}
	out = make([]byte, size)

	// trying the constanttimecopy
	subtle.ConstantTimeCopy(1, out[size-n:], input)
	return
}

// the DUDECT functions :
func prepare_inputs(input_data [][]byte, classes []int) {
	//fmt.Println("Preparing input")
	rn := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	var ones, zeros int
	f, err := os.OpenFile("ones.txt", os.O_RDWR|os.O_APPEND, 0660)
	defer f.Close()
	if err != nil {
		log.Fatal(err)
	}
	for i := 0; i < number_measurements; i++ {
		classes[i] = rn.Intn(2)
		lowerB := new(big.Int).Lsh(bigOne, 2047)
		tmp := new(big.Int).Rand(rn, lowerB)
		tmp.Add(tmp, lowerB)
		tmp.Mod(tmp, test2048Key.N)

		m, err := decrypt(nil, test2048Key, tmp)
		data := tmp.Bytes()
		if err == nil {
			err = fmt.Errorf("decryption successful: %s", m.Text(16))
		} else if err.Error() == "cbiggern" {
			classes[i] = 1
			var erre error
			data, erre = EncryptOAEP(sha256.New(), rand.Reader, &test2048Key.PublicKey, data[len(data)/2+5:], []byte(""))
			if erre != nil {
				log.Fatal(erre)
			}
		}

		if classes[i] == 0 { // wrong padding
			zeros++
		} else {
			classes[i] = 1 // this is the class of the input which need padding
			var erre error
			data, erre = EncryptOAEP(sha256.New(), rand.Reader, &test2048Key.PublicKey, data[len(data)/2+5:], []byte(""))
			if erre != nil {
				log.Fatal(erre)
			}
			ones++
			//	_, err = f.WriteString(hex.EncodeToString(data) + "\n")
			//	if err != nil {
			//		log.Fatal(err)
			//}
			//fmt.Println(data)
		}

		input_data[i] = data
	}
	f.Sync()
	fmt.Println("And we have:", ones, " 1s and ", zeros, " 0s.")
	return
}

func do_one_computation(data []byte) {

	p, err := DecryptOAEP(sha256.New(), rand.Reader, test2048Key, data, []byte(""))
	if err == nil {
		err = fmt.Errorf("decryption successful: %s", string(p))
	}
}

/*

// For the leftPad test:
func prepare_inputs(input_data [][]byte, classes []int) {
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
			input_data[i] = data
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
*/
