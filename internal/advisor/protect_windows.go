//go:build windows

package advisor

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// entropy ties the encrypted key to Pare, so other DPAPI blobs of the same user are not
// accepted as Pare settings and vice versa.
var entropy = []byte("Pare advisor settings v1")

func protect(plain []byte) ([]byte, error) {
	return dpapi(plain, true)
}

func unprotect(sealed []byte) ([]byte, error) {
	return dpapi(sealed, false)
}

func dpapi(input []byte, encrypt bool) ([]byte, error) {
	if len(input) == 0 {
		return nil, nil
	}
	in := windows.DataBlob{Size: uint32(len(input)), Data: &input[0]}
	extra := windows.DataBlob{Size: uint32(len(entropy)), Data: &entropy[0]}
	var out windows.DataBlob
	var err error
	if encrypt {
		err = windows.CryptProtectData(&in, nil, &extra, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&in, nil, &extra, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
