package openssl

// #include "shim.h"
import "C"
import (
	"errors"
	"unsafe"
)

/// VerifyECDSASignature verifies data valid against an ECDSA signature and ECDSA Public Key
/// - Parameter publicKey: The OpenSSL EVP_PKEY ECDSA key
/// - Parameter signature: The ECDSA signature to verify
/// - Parameter data: The data used to generate the signature
/// - Returns: True if the signature was verified
func VerifyECDSASignature(publicKey, signature, data []byte) (bool, error) {
	ecsig := C.ECDSA_SIG_new()
	defer C.ECDSA_SIG_free(ecsig)
	sigData := signature

	C.BN_bin2bn((*C.uchar)(&sigData[0]), 32, ecsig.r)
	C.BN_bin2bn((*C.uchar)(&sigData[32]), 32, ecsig.s)

	sigSize := C.i2d_ECDSA_SIG(ecsig, nil)

	derBytes := (*C.uchar)(C.malloc(C.size_t(sigSize)))
	defer C.free(unsafe.Pointer(derBytes))

	// ignoring result, because it is the same as sigSize
	C.i2d_ECDSA_SIG(ecsig, &derBytes)

	// read EC Public Key
	inf := C.BIO_new(C.BIO_s_mem())
	if inf == nil {
		return false, errors.New("failed allocating input buffer")
	}
	defer C.BIO_free(inf)
	_, err := asAnyBio(inf).Write(publicKey)
	if err != nil {
		return false, err
	}

	eckey := C.d2i_EC_PUBKEY_bio(inf, nil)
	if eckey == nil {
		return false, errors.New("failed to load ec public key")
	}
	defer C.EC_KEY_free(eckey)

	out := C.BIO_new(C.BIO_s_mem())
	if out == nil {
		return false, errors.New("failed allocating output buffer")
	}
	defer C.BIO_free(out)
	i := C.PEM_write_bio_EC_PUBKEY(out, eckey)
	if i != 1 {
		return false, errors.New("failed to write bio ec public key")
	}
	pemKey := C.PEM_read_bio_PUBKEY(out, nil, nil, nil)
	defer C.EVP_PKEY_free(pemKey)

	keyType := C.EVP_PKEY_base_id(pemKey)
	// TODO: support other key types such as RSA, DSA, etc.
	if keyType != C.EVP_PKEY_EC {
		return false, errors.New("public key is incorrect type")
	}

	ctx := &C.EVP_MD_CTX{}
	ctxPointer := unsafe.Pointer(ctx)
	bmd := C.BIO_new(C.BIO_f_md())
	defer C.BIO_free(bmd)

	if C.BIO_ctrl(bmd, C.BIO_C_GET_MD_CTX, 0, ctxPointer) != 1 {
		return false, errors.New("error getting context")
	}

	nRes := C.EVP_DigestVerifyInit(ctx, nil, nil, nil, pemKey)
	if nRes != 1 {
		return false, errors.New("unable to init digest verify")
	}
	defer C.EVP_MD_CTX_cleanup(ctx)

	nRes = C.EVP_DigestUpdate(ctx, unsafe.Pointer((*C.uchar)(&data[0])), C.size_t(len(data)))
	if nRes != 1 {
		return false, errors.New("unable to update digest")
	}

	nRes = C.EVP_DigestVerifyFinal(ctx, derBytes, C.size_t(sigSize))
	if nRes != 1 {
		return false, nil
	}

	return true, nil
}