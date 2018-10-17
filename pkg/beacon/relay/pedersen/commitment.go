// Package pedersen implements a Verifiable Secret Sharing (VSS) scheme described
// by Torben Pryds Pedersen in the referenced [Ped91b] paper.
// It consists of VSS parameters structure and functions to calculate and verify
// a commitment to chosen value.
//
// Commitment scheme allows a party (Commiter) to commit to a chosen value while
// keeping the value hidden from the other party (Verifier).
// On verification stage Committer reveals the value along with a DecommitmentKey,
// so Verifier can confirm the revealed value matches the committed one.
//
// pedersen.NewVSS() initializes scheme with `g` and `h` values, which need to
// be randomly generated for each scheme execution.
// To stop an adversary Committer from changing the value them already committed
// to, the scheme requires that `log_g(h)` is unknown to the Committer.
//
// You may consult our documentation for more details:
// docs/cryptography/trapdoor-commitments.html#_pedersen_commitment
//
//     [Ped91b]: T. Pedersen. Non-interactive and information-theoretic secure
//         verifiable secret sharing. In: Advances in Cryptology — Crypto '91,
//         pages 129-140. LNCS No. 576.
//         https://www.cs.cornell.edu/courses/cs754/2001fa/129.PDF
//     [GJKR 99]: Gennaro R., Jarecki S., Krawczyk H., Rabin T. (1999) Secure
//         Distributed Key Generation for Discrete-Log Based Cryptosystems. In:
//         Stern J. (eds) Advances in Cryptology — EUROCRYPT ’99. EUROCRYPT 1999.
//         Lecture Notes in Computer Science, vol 1592. Springer, Berlin, Heidelberg
//         http://groups.csail.mit.edu/cis/pubs/stasio/vss.ps.gz
package pedersen

import (
	crand "crypto/rand"
	"fmt"
	"math/big"

	"github.com/keep-network/keep-core/pkg/internal/byteutils"
)

// VSS scheme parameters
type VSS struct {
	// p, q are primes such that `p = 2q + 1`.
	p, q *big.Int

	// g and h are elements of a group of order q, and should be chosen such that
	// no one knows log_g(h).
	g, h *big.Int
}

// Commitment represents a single commitment to a single message. One is produced
// for each message we have committed to.
//
// It is usually shared with the verifier immediately after it has been produced
// and lets the recipient verify if the message revealed later by the committing
// party is really what that party has committed to.
//
// The commitment itself is not enough for a verification. In order to perform
// a verification, the interested party must receive the `DecommitmentKey`.
type Commitment struct {
	vss        *VSS
	commitment *big.Int
}

// DecommitmentKey represents the key that allows a recipient to open an
// already-received commitment and verify if the value is what the sender have
// really committed to.
type DecommitmentKey struct {
	t *big.Int
}

// NewVSS generates parameters for a scheme execution
func NewVSS(p, q *big.Int) (*VSS, error) {
	if !p.ProbablyPrime(20) || !q.ProbablyPrime(20) {
		return nil, fmt.Errorf("p and q have to be primes")
	}

	// We need to check that `q^2` does not divide `p - 1`.
	// This is required for `h` to be an element in a group generated by `g`.
	// See [GJKR 99] section 4.2.
	modulus := new(big.Int).Mod(
		new(big.Int).Sub(p, big.NewInt(1)),
		new(big.Int).Exp(q, big.NewInt(2), nil),
	)
	if modulus.Cmp(big.NewInt(0)) == 0 {
		return nil, fmt.Errorf("incorrect p and q values")
	}

	// Generate random `g`
	randomG, err := randomFromZn(p)
	if err != nil {
		return nil, fmt.Errorf("g generation failed [%s]", err)
	}
	g := new(big.Int).Exp(randomG, big.NewInt(2), nil) // (randomZ(0, 2^p - 1]) ^2

	// Generate `h` jointly by the players as described in section 4.2 of [GJKR 99]
	// First players have to jointly generate a random value r ∈ Z*_p with coin
	// flipping protocol.
	// To generate a random element `h` in a subgroup generated by `g` one needs
	// to calculate `h = r^k mod p` where `k = (p - 1) / q`
	randomValue, err := randomFromZn(p) // TODO this should be generated with coin flipping protocol
	if err != nil {
		return nil, fmt.Errorf("randomValue generation failed [%s]", err)
	}

	k := new(big.Int).Div(
		new(big.Int).Sub(p, big.NewInt(1)),
		q,
	)

	h := new(big.Int).Exp(randomValue, k, p)

	return &VSS{p: p, q: q, g: g, h: h}, nil
}

// CommitmentTo takes a secret message and a set of parameters and returns
// a commitment to that message and the associated decommitment key.
//
// First random `r` value is chosen as a Decommitment Key.
// Then commitment is calculated as `(g ^ digest) * (h ^ r) mod p`, where digest
// is sha256 hash of the secret brought to big.Int.
func (vss *VSS) CommitmentTo(secret []byte) (*Commitment, *DecommitmentKey, error) {
	r, err := randomFromZn(vss.q) // randomZ(0, 2^q - 1]
	if err != nil {
		return nil, nil, fmt.Errorf("r generation failed [%s]", err)
	}

	digest := calculateDigest(secret, vss.q)
	commitment := CalculateCommitment(vss, digest, r)

	return &Commitment{vss, commitment},
		&DecommitmentKey{r},
		nil
}

// Verify checks the received commitment against the revealed secret message.
func (c *Commitment) Verify(decommitmentKey *DecommitmentKey, secret []byte) bool {
	digest := calculateDigest(secret, c.vss.q)
	expectedCommitment := CalculateCommitment(c.vss, digest, decommitmentKey.t)
	return expectedCommitment.Cmp(c.commitment) == 0
}

func calculateDigest(secret []byte, mod *big.Int) *big.Int {
	hash := byteutils.Sha256Sum(secret)
	digest := new(big.Int).Mod(hash, mod)
	return digest
}

// CalculateCommitment calculates a commitment with equation `(g ^ s) * (h ^ r) mod p`
// where:
// - `g` and `h` are scheme specific parameters passed in vss,
// - `s` is a message to which one is committing,
// - `r` is a decommitment key.
func CalculateCommitment(vss *VSS, digest, r *big.Int) *big.Int {
	return new(big.Int).Mod(
		new(big.Int).Mul(
			new(big.Int).Exp(vss.g, digest, vss.p),
			new(big.Int).Exp(vss.h, r, vss.p),
		),
		vss.p,
	)
}

// randomFromZn generates a random `big.Int` in a range (0, 2^n - 1]
func randomFromZn(n *big.Int) (*big.Int, error) {
	x := big.NewInt(0)
	var err error
	// TODO check if this is what we really need for g,h and r
	// 2^n - 1
	max := new(big.Int).Sub(
		// new(big.Int).Exp(big.NewInt(2), n, nil),
		n,
		big.NewInt(1),
	)
	for x.Sign() == 0 {
		x, err = crand.Int(crand.Reader, max)
		if err != nil {
			return nil, fmt.Errorf("failed to generate random number [%s]", err)
		}
	}
	return x, nil
}
