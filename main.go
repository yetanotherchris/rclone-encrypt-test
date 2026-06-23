package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/yetanotherchris/rclone-encrypt-test/internal/crypt"
)

var (
	version = "dev"

	passwordFlag     string
	saltFlag         string
	filenameEncoding string
	inputFile        string
	outputFile       string
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "rclone-encrypt-test",
		Short:   "Encrypt and decrypt files using rclone crypt format",
		Version: version,
	}

	encryptCmd := &cobra.Command{
		Use:   "encrypt",
		Short: "Encrypt a file",
		RunE:  runEncrypt,
	}
	decryptCmd := &cobra.Command{
		Use:   "decrypt",
		Short: "Decrypt a file",
		RunE:  runDecrypt,
	}

	for _, c := range []*cobra.Command{encryptCmd, decryptCmd} {
		c.Flags().StringVarP(&inputFile, "input-file", "i", "", "Input file (required)")
		c.Flags().StringVarP(&outputFile, "output-file", "o", "", "Output file (optional)")
		c.Flags().StringVar(&passwordFlag, "password", "", "Password (insecure: may appear in shell history; prefer prompt or RCLONE_ENCRYPT_PASSWORD env var)")
		c.Flags().StringVar(&saltFlag, "salt", "", "Optional salt (if not provided, rclone default is used)")
		c.Flags().StringVar(&filenameEncoding, "filename-encoding", "base32", "Filename encoding: base32 (default), base64, base32768")
		_ = c.MarkFlagRequired("input-file")
	}

	rootCmd.AddCommand(encryptCmd, decryptCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(pw), nil
}

func getPasswordAndSalt(isEncrypt bool) (string, string, error) {
	pw := passwordFlag
	if pw == "" {
		if env := os.Getenv("RCLONE_ENCRYPT_PASSWORD"); env != "" {
			pw = env
		}
	}
	if pw == "" {
		var err error
		pw, err = readPassword("Password: ")
		if err != nil {
			return "", "", err
		}
		if isEncrypt {
			confirm, err := readPassword("Confirm password: ")
			if err != nil {
				return "", "", err
			}
			if pw != confirm {
				return "", "", fmt.Errorf("passwords do not match")
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, "WARNING: --password exposes the password in process list/shell history.")
		fmt.Fprintln(os.Stderr, "Use an environment variable (RCLONE_ENCRYPT_PASSWORD) or the interactive prompt instead.")
		fmt.Fprintln(os.Stderr, "After use, clear your shell history entry (e.g. history -d <line> on bash).")
	}

	salt := saltFlag
	if salt == "" {
		if env := os.Getenv("RCLONE_ENCRYPT_SALT"); env != "" {
			salt = env
		} else if passwordFlag == "" {
			s, err := readPassword("Salt (optional, press Enter to use rclone default): ")
			if err != nil {
				return "", "", err
			}
			salt = strings.TrimSpace(s)
		}
	}
	return pw, salt, nil
}

func getCipher(pw, salt, enc string) (*crypt.Cipher, error) {
	if enc == "" {
		enc = "base32"
	}
	return crypt.NewCipher(pw, salt, enc)
}

func runEncrypt(cmd *cobra.Command, args []string) error {
	pw, salt, err := getPasswordAndSalt(true)
	if err != nil {
		return err
	}
	c, err := getCipher(pw, salt, filenameEncoding)
	if err != nil {
		return err
	}

	inData, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	plainName := filepath.Base(inputFile)
	encName := c.EncryptFileName(plainName)

	outPath := outputFile
	if outPath == "" {
		outPath = encName
	}

	plainR := bytesReader(inData)
	encR, err := c.EncryptData(plainR)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, encR); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Encrypted to %s (filename encoding: %s)\n", outPath, filenameEncoding)
	return nil
}

func runDecrypt(cmd *cobra.Command, args []string) error {
	pw, salt, err := getPasswordAndSalt(false)
	if err != nil {
		return err
	}
	c, err := getCipher(pw, salt, filenameEncoding)
	if err != nil {
		return err
	}

	f, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer f.Close()

	decR, err := c.DecryptData(f)
	if err != nil {
		return fmt.Errorf("decrypt data: %w", err)
	}

	encName := filepath.Base(inputFile)
	decName, err := c.DecryptFileName(encName)
	if err != nil {
		// If name decrypt fails, still write content but warn; use input basename without ext or .dec
		fmt.Fprintf(os.Stderr, "warning: could not decrypt filename %q: %v\n", encName, err)
		decName = ""
	}

	outPath := outputFile
	if outPath == "" {
		if decName != "" {
			outPath = decName
		} else {
			outPath = inputFile + ".dec"
		}
	}

	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, decR); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Decrypted to %s\n", outPath)
	return nil
}

func bytesReader(b []byte) io.Reader {
	return &byteReader{b: b}
}

type byteReader struct {
	b []byte
	i int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}
