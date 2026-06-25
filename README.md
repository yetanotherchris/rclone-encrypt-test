# rclone-encrypt-test-grok-go

A small CLI tool that encrypts and decrypts using the rclone encryption defaults. 

Rclone uses a custom salt if no salt is provided, which this tool will use by default. A few similar tools:

- https://github.com/rclone/rclone
- https://github.com/mcolatosti/rclonedecrypt
- https://github.com/br0kenpixel/rclone-rcc
- @fyears/rclone-crypt

Rclone encryption uses: 
- NaCl SecretBox (XSalsa20 + Poly1305) for the file contents.
- AES256 for the filenames.
- scrypt for keymaterial.

## Installation

**Homebrew (macOS/Linux)**

```bash
brew tap yetanotherchris/rclone-encrypt-test https://github.com/yetanotherchris/rclone-encrypt-test
brew install rclone-encrypt-test
```

**Scoop (Windows)**

```bash
scoop bucket add rclone-encrypt-test https://github.com/yetanotherchris/rclone-encrypt-test
scoop install rclone-encrypt-test
```

## Usage

Encrypt a file (will prompt for password and optional salt):

```bash
rclone-encrypt-test encrypt -i plaintext.txt
```

Decrypt a file:

```bash
rclone-encrypt-test decrypt -i <encrypted-filename>
```

Specify input and output explicitly:

```bash
rclone-encrypt-test encrypt -i secret.txt -o encrypted.bin
rclone-encrypt-test decrypt -i encrypted.bin -o recovered.txt
```

Use a custom filename encoding (base32 is the rclone default):

```bash
rclone-encrypt-test encrypt -i file.txt --filename-encoding base64
rclone-encrypt-test decrypt -i <base64-encrypted-name> --filename-encoding base64
```

Provide password via flag (insecure, shows in history/process list):

```bash
rclone-encrypt-test encrypt -i file.txt --password 'p@ss'
```

**Security warning**: Using `--password` may leave the password in your shell history and process listings. Prefer the interactive prompt or the environment variable `RCLONE_ENCRYPT_PASSWORD`. After using the flag, clear the relevant history entry (e.g. `history -d <line>` in bash, or use a leading space to avoid history in some shells).

Provide a salt (optional; different salt == different key):

```bash
rclone-encrypt-test encrypt -i file.txt --salt 'optional-salt-value'
```

Environment variables (used when flags/prompts are not supplied):

```bash
export RCLONE_ENCRYPT_PASSWORD='secret'
export RCLONE_ENCRYPT_SALT='optional'
rclone-encrypt-test decrypt -i <encrypted>
```

## Building from Source

Requires Go 1.25+.

```bash
git clone https://github.com/yetanotherchris/rclone-encrypt-test
cd rclone-encrypt-test
go build -o rclone-encrypt-test .
```

## Releases

Pushing a `vX.Y.Z` tag triggers the Build and Release workflow which cross-compiles for Linux/macOS/Windows, publishes a GitHub Release, and updates the Scoop and Homebrew manifests.

## Testing

```bash
go test -v ./...
```

## License

MIT
