// Copyright 2017 The PlatON Authors
// This file is part of the PlatON library.
//
// The PlatON library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The PlatON library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the PlatON library. If not, see <http://www.gnu.org/licenses/>.

// +build !nacl,!js,!nocgo

package vrf

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"math/big"

	"github.com/hashkey-chain/hashkey-chain/crypto/rfc6979"
	"github.com/hashkey-chain/hashkey-chain/crypto/secp256k1"
)

const (
	limit = 1000
	N2    = 32 // ceil(log2(q) / 8)
	N     = N2 / 2
)

var (
	ErrMalformedInput = errors.New("ECVRF: malformed input")
	ErrDecodeError    = errors.New("ECVRF: decode error")
	ErrInternalError  = errors.New("ECVRF: internal error")
	curve             = secp256k1.S256()
	gx, gy            = G()
)

// assume <pk, sk> were generated by ed25519.GenerateKey()
func eCVRF_prove(pk []byte, sk []byte, m []byte) (pi []byte, err error) {
	hx, hy := ECVRF_hash_to_curve(m, pk)
	r := ECP2OS(curve.ScalarMult(hx, hy, sk))
	k, err := rfc6979.ECVRF_nonce_generation(sk, m)
	if err != nil {
		panic(err)
	}
	kp := ECP2OS(k.PublicKey.X, k.PublicKey.Y)

	// ECVRF_hash_points(g, h, g^x, h^x, g^k, h^k)
	c := ECVRF_hash_points(ECP2OS(gx, gy), ECP2OS(hx, hy), pk,
		r, kp, ECP2OS(curve.ScalarMult(hx, hy, k.D.Bytes())))

	// s = k - c*q mod q
	var z big.Int
	var xx = new(big.Int).SetBytes(sk)
	s := z.Mod(z.Sub(k.D, z.Mul(c, xx)), curve.N)

	// pi = gamma || I2OSP(c, N) || I2OSP(s, 2N)
	var buf bytes.Buffer
	buf.Write(r)            // 2N
	buf.Write(I2OSP(c, N))  //BigEndian
	buf.Write(I2OSP(s, N2)) //BigEndian
	return buf.Bytes(), nil
}

func eCVRF_proof2hash(pi []byte) []byte {
	return pi[1 : N2+1]
}

func eCVRF_verify(pk []byte, pi []byte, m []byte) (bool, error) {
	gmx, gmy, c, s, err := ECVRF_decode_proof(pi)
	if err != nil {
		return false, err
	}

	// u = (g^x)^c * g^s = P^c * g^s
	px, py := OS2ECP(pk)
	if px == nil || py == nil {
		return false, ErrMalformedInput
	}

	cc := c.Bytes()
	ss := s.Bytes()

	x1, y1 := curve.ScalarMult(px, py, cc)
	x2, y2 := curve.ScalarBaseMult(ss)
	ux, uy := curve.Add(x1, y1, x2, y2)

	hx, hy := ECVRF_hash_to_curve(m, pk)

	// v = gamma^c * h^s
	//	fmt.Printf("c, r, s, h\n%s%s%s%s\n", hex.Dump(c[:]), hex.Dump(ECP2OS(r)), hex.Dump(s[:]), hex.Dump(ECP2OS(h)))
	x3, y3 := curve.ScalarMult(gmx, gmy, cc)
	x4, y4 := curve.ScalarMult(hx, hy, ss)
	vx, vy := curve.Add(x3, y3, x4, y4)

	// c' = ECVRF_hash_points(g, h, g^x, gamma, u, v)
	c2 := ECVRF_hash_points(ECP2OS(gx, gy), ECP2OS(hx, hy), pk, ECP2OS(gmx, gmy), ECP2OS(ux, uy), ECP2OS(vx, vy))

	return c2.Cmp(c) == 0, nil
}

func ECVRF_decode_proof(pi []byte) (x *big.Int, y *big.Int, c *big.Int, s *big.Int, err error) {
	i := 0
	x, y = OS2ECP(pi[0 : N2+1])
	i += N2 + 1
	if x == nil || y == nil {
		return nil, nil, nil, nil, ErrDecodeError
	}

	c = OS2IP(pi[i : i+N])
	i += N
	s = OS2IP(pi[i : i+N2])
	return
}

func ECVRF_hash_points(ps ...[]byte) *big.Int {
	h := sha256.New()
	//	fmt.Printf("hash_points:\n")
	for _, p := range ps {
		h.Write(p)
		//		fmt.Printf("%s\n", hex.Dump(p))
	}
	v := h.Sum(nil)
	return OS2IP(v[:N])
}

func ECVRF_hash_to_curve(m []byte, pk []byte) (x, y *big.Int) {
	hash := sha256.New()
	for i := int64(0); i < limit; i++ {
		ctr := I2OSP(big.NewInt(i), 4)
		hash.Write(m)
		hash.Write(pk)
		hash.Write(ctr)
		h := hash.Sum(nil)
		hash.Reset()
		var buf [33]byte
		buf[0] = 0x2
		copy(buf[1:], h)
		x, y = OS2ECP(buf[:])
		if x != nil || y != nil {
			return
		}
	}
	panic("ECVRF_hash_to_curve: couldn't make a point on curve")
}

func OS2ECP(os []byte) (Bx, By *big.Int) {
	return secp256k1.DecompressPubkey(os)
}

func ECP2OS(Bx, By *big.Int) []byte {
	return secp256k1.CompressPubkey(Bx, By)
}

func I2OSP(b *big.Int, n int) []byte {
	os := b.Bytes()
	if n > len(os) {
		var buf bytes.Buffer
		buf.Write(make([]byte, n-len(os))) // prepend 0s
		buf.Write(os)
		return buf.Bytes()
	} else {
		return os[:n]
	}
}

func OS2IP(os []byte) *big.Int {
	return new(big.Int).SetBytes(os)
}

func G() (x, y *big.Int) {
	var one = new(big.Int).SetInt64(1)
	x, y = curve.ScalarBaseMult(one.Bytes()) // g = g^1
	return
}
