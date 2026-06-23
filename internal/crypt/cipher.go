package crypt

import (
	"bytes"
	"crypto/aes"
	gocipher "crypto/cipher"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/Max-Sum/base32768"
	"github.com/yetanotherchris/rclone-encrypt-test/internal/eme"
	"github.com/yetanotherchris/rclone-encrypt-test/internal/pkcs7"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/crypto/scrypt"
)

const (
	nameCipherBlockSize = aes.BlockSize
	fileMagic           = "RCLONE\x00\x00"
	fileMagicSize       = len(fileMagic)
	fileNonceSize       = 24
	fileHeaderSize      = fileMagicSize + fileNonceSize
	blockHeaderSize     = secretbox.Overhead
	blockDataSize       = 64 * 1024
	blockSize           = blockHeaderSize + blockDataSize
)

var (
	defaultSalt                = []byte{0xA8, 0x0D, 0xF4, 0x3A, 0x8F, 0xBD, 0x03, 0x08, 0xA7, 0xCA, 0xB8, 0x3E, 0x58, 0x1F, 0x86, 0xB1}
	fileMagicBytes             = []byte(fileMagic)
	ErrorBadDecrypt            = errors.New("bad decryption")
	ErrorEncryptedFileTooShort = errors.New("file is too short to be encrypted")
	ErrorEncryptedBadMagic     = errors.New("not an encrypted file - bad magic string")
	ErrorEncryptedBadBlock     = errors.New("failed to authenticate decrypted block - bad password?")
	ErrorNotAnEncryptedFile    = errors.New("not an encrypted file")
	ErrorBadBase32Encoding     = errors.New("bad base32 filename encoding")
)

type fileNameEncoding interface {
	EncodeToString(src []byte) string
	DecodeString(s string) ([]byte, error)
}

type caseInsensitiveBase32Encoding struct{}

func (caseInsensitiveBase32Encoding) EncodeToString(src []byte) string {
	encoded := base32.HexEncoding.EncodeToString(src)
	encoded = strings.TrimRight(encoded, "=")
	return strings.ToLower(encoded)
}

func (caseInsensitiveBase32Encoding) DecodeString(s string) ([]byte, error) {
	if strings.HasSuffix(s, "=") {
		return nil, ErrorBadBase32Encoding
	}
	roundUpToMultipleOf8 := (len(s) + 7) &^ 7
	equals := roundUpToMultipleOf8 - len(s)
	s = strings.ToUpper(s) + "========"[:equals]
	return base32.HexEncoding.DecodeString(s)
}

func NewNameEncoding(s string) (fileNameEncoding, error) {
	s = strings.ToLower(s)
	switch s {
	case "base32", "":
		return caseInsensitiveBase32Encoding{}, nil
	case "base64":
		return base64.RawURLEncoding, nil
	case "base32768":
		return base32768.SafeEncoding, nil
	default:
		return nil, fmt.Errorf("unknown file name encoding %q", s)
	}
}

type Cipher struct {
	dataKey     [32]byte
	nameKey     [32]byte
	nameTweak   [nameCipherBlockSize]byte
	block       gocipher.Block
	fileNameEnc fileNameEncoding
	buffers     sync.Pool
	cryptoRand  io.Reader
}

func NewCipher(password, salt string, filenameEncoding string) (*Cipher, error) {
	enc, err := NewNameEncoding(filenameEncoding)
	if err != nil {
		return nil, err
	}
	c := &Cipher{
		fileNameEnc: enc,
		cryptoRand:  rand.Reader,
	}
	c.buffers.New = func() any { return new([blockSize]byte) }
	if err := c.Key(password, salt); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Cipher) Key(password, salt string) error {
	const keySize = len(c.dataKey) + len(c.nameKey) + len(c.nameTweak)
	saltBytes := defaultSalt
	if salt != "" {
		saltBytes = []byte(salt)
	}
	var key []byte
	var err error
	if password == "" {
		key = make([]byte, keySize)
	} else {
		key, err = scrypt.Key([]byte(password), saltBytes, 16384, 8, 1, keySize)
		if err != nil {
			return err
		}
	}
	copy(c.dataKey[:], key)
	copy(c.nameKey[:], key[len(c.dataKey):])
	copy(c.nameTweak[:], key[len(c.dataKey)+len(c.nameKey):])
	c.block, err = aes.NewCipher(c.nameKey[:])
	return err
}

func (c *Cipher) getBlock() *[blockSize]byte    { return c.buffers.Get().(*[blockSize]byte) }
func (c *Cipher) putBlock(buf *[blockSize]byte) { c.buffers.Put(buf) }

func (c *Cipher) encryptSegment(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	padded := pkcs7.Pad(nameCipherBlockSize, []byte(plaintext))
	ciphertext := eme.Transform(c.block, c.nameTweak[:], padded, eme.DirectionEncrypt)
	return c.fileNameEnc.EncodeToString(ciphertext)
}

func (c *Cipher) decryptSegment(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	raw, err := c.fileNameEnc.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	if len(raw)%nameCipherBlockSize != 0 {
		return "", errors.New("not a multiple of blocksize")
	}
	if len(raw) == 0 {
		return "", errors.New("too short after decode")
	}
	padded := eme.Transform(c.block, c.nameTweak[:], raw, eme.DirectionDecrypt)
	plain, err := pkcs7.Unpad(nameCipherBlockSize, padded)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (c *Cipher) EncryptFileName(in string) string {
	segments := strings.Split(in, "/")
	for i := range segments {
		segments[i] = c.encryptSegment(segments[i])
	}
	return strings.Join(segments, "/")
}

func (c *Cipher) DecryptFileName(in string) (string, error) {
	segments := strings.Split(in, "/")
	for i := range segments {
		var err error
		segments[i], err = c.decryptSegment(segments[i])
		if err != nil {
			return "", err
		}
	}
	return strings.Join(segments, "/"), nil
}

type nonce [fileNonceSize]byte

func (n *nonce) pointer() *[fileNonceSize]byte { return (*[fileNonceSize]byte)(n) }

func (n *nonce) fromReader(in io.Reader) error {
	read, err := io.ReadFull(in, (*n)[:])
	if read != fileNonceSize {
		return fmt.Errorf("short read of nonce: %w", err)
	}
	return nil
}

func (n *nonce) fromBuf(buf []byte) { copy((*n)[:], buf) }

func (n *nonce) increment() {
	for i := range n {
		n[i]++
		if n[i] != 0 {
			break
		}
	}
}

type encrypter struct {
	mu       sync.Mutex
	in       io.Reader
	c        *Cipher
	nonce    nonce
	buf      *[blockSize]byte
	readBuf  *[blockSize]byte
	bufIndex int
	bufSize  int
	err      error
}

func (c *Cipher) newEncrypter(in io.Reader, nonce *nonce) (*encrypter, error) {
	fh := &encrypter{
		in:      in,
		c:       c,
		buf:     c.getBlock(),
		readBuf: c.getBlock(),
		bufSize: fileHeaderSize,
	}
	if nonce != nil {
		fh.nonce = *nonce
	} else {
		if err := fh.nonce.fromReader(c.cryptoRand); err != nil {
			return nil, err
		}
	}
	copy((*fh.buf)[:], fileMagicBytes)
	copy((*fh.buf)[fileMagicSize:], fh.nonce[:])
	return fh, nil
}

func (fh *encrypter) Read(p []byte) (n int, err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.err != nil {
		return 0, fh.err
	}
	if fh.bufIndex >= fh.bufSize {
		readBuf := (*fh.readBuf)[:blockDataSize]
		nr, rerr := io.ReadFull(fh.in, readBuf)
		if nr == 0 {
			return fh.finish(rerr)
		}
		secretbox.Seal((*fh.buf)[:0], readBuf[:nr], fh.nonce.pointer(), &fh.c.dataKey)
		fh.bufIndex = 0
		fh.bufSize = blockHeaderSize + nr
		fh.nonce.increment()
		if rerr != nil && rerr != io.EOF && rerr != io.ErrUnexpectedEOF {
			fh.err = rerr
		}
	}
	n = copy(p, (*fh.buf)[fh.bufIndex:fh.bufSize])
	fh.bufIndex += n
	return n, nil
}

func (fh *encrypter) finish(err error) (int, error) {
	if fh.err != nil {
		return 0, fh.err
	}
	fh.err = err
	fh.c.putBlock(fh.buf)
	fh.buf = nil
	fh.c.putBlock(fh.readBuf)
	fh.readBuf = nil
	return 0, err
}

func (c *Cipher) EncryptData(in io.Reader) (io.Reader, error) {
	out, err := c.newEncrypter(in, nil)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type decrypter struct {
	mu       sync.Mutex
	rc       io.ReadCloser
	nonce    nonce
	c        *Cipher
	buf      *[blockSize]byte
	readBuf  *[blockSize]byte
	bufIndex int
	bufSize  int
	err      error
}

func (c *Cipher) newDecrypter(rc io.ReadCloser) (*decrypter, error) {
	fh := &decrypter{
		rc:      rc,
		c:       c,
		buf:     c.getBlock(),
		readBuf: c.getBlock(),
	}
	readBuf := (*fh.readBuf)[:fileHeaderSize]
	n, err := io.ReadFull(fh.rc, readBuf)
	if n < fileHeaderSize && err == io.EOF {
		_ = fh.finishAndClose(ErrorEncryptedFileTooShort)
		return nil, ErrorEncryptedFileTooShort
	} else if err != nil && err != io.EOF {
		_ = fh.finishAndClose(err)
		return nil, err
	}
	if !bytes.Equal(readBuf[:fileMagicSize], fileMagicBytes) {
		_ = fh.finishAndClose(ErrorEncryptedBadMagic)
		return nil, ErrorEncryptedBadMagic
	}
	fh.nonce.fromBuf(readBuf[fileMagicSize:])
	return fh, nil
}

func (fh *decrypter) fillBuffer() error {
	readBuf := fh.readBuf
	n, err := io.ReadFull(fh.rc, (*readBuf)[:])
	if n == 0 {
		return err
	}
	if n <= blockHeaderSize {
		if err != nil && err != io.EOF {
			return err
		}
		return errors.New("truncated block header")
	}
	_, ok := secretbox.Open((*fh.buf)[:0], (*readBuf)[:n], fh.nonce.pointer(), &fh.c.dataKey)
	if !ok {
		if err != nil && err != io.EOF {
			return err
		}
		return ErrorEncryptedBadBlock
	}
	fh.bufIndex = 0
	fh.bufSize = n - blockHeaderSize
	fh.nonce.increment()
	return nil
}

func (fh *decrypter) Read(p []byte) (n int, err error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.err != nil {
		return 0, fh.err
	}
	if fh.bufIndex >= fh.bufSize {
		err = fh.fillBuffer()
		if err != nil {
			return 0, fh.finish(err)
		}
	}
	n = copy(p, (*fh.buf)[fh.bufIndex:fh.bufSize])
	fh.bufIndex += n
	return n, nil
}

func (fh *decrypter) finish(err error) error {
	if fh.err != nil {
		return fh.err
	}
	fh.err = err
	fh.c.putBlock(fh.buf)
	fh.buf = nil
	fh.c.putBlock(fh.readBuf)
	fh.readBuf = nil
	return err
}

func (fh *decrypter) finishAndClose(err error) error {
	_ = fh.finish(err)
	if fh.rc != nil {
		_ = fh.rc.Close()
	}
	return err
}

func (fh *decrypter) Close() error {
	fh.mu.Lock()
	defer fh.mu.Unlock()
	if fh.err == ErrorFileClosed {
		return fh.err
	}
	if fh.err == nil {
		_ = fh.finish(io.EOF)
	}
	fh.err = ErrorFileClosed
	if fh.rc == nil {
		return nil
	}
	return fh.rc.Close()
}

var ErrorFileClosed = errors.New("file already closed")

func (c *Cipher) DecryptData(rc io.ReadCloser) (io.ReadCloser, error) {
	return c.newDecrypter(rc)
}

func (c *Cipher) EncryptedSize(size int64) int64 {
	blocks, residue := size/blockDataSize, size%blockDataSize
	encryptedSize := int64(fileHeaderSize) + blocks*(blockHeaderSize+blockDataSize)
	if residue != 0 {
		encryptedSize += blockHeaderSize + residue
	}
	return encryptedSize
}

func (c *Cipher) DecryptedSize(size int64) (int64, error) {
	size -= int64(fileHeaderSize)
	if size < 0 {
		return 0, ErrorEncryptedFileTooShort
	}
	blocks, residue := size/blockSize, size%blockSize
	decryptedSize := blocks * blockDataSize
	if residue != 0 {
		residue -= blockHeaderSize
		if residue <= 0 {
			return 0, errors.New("bad block header")
		}
	}
	decryptedSize += residue
	return decryptedSize, nil
}
