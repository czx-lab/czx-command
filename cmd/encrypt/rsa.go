package encrypt

import (
	"command/cmd"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"

	"github.com/spf13/cobra"
)

type RSA struct {
	format   string
	encoding string
	bits     int
	outDir   string
}

func NewRSA() *RSA {
	return &RSA{}
}

// Command implements cmd.ICommand.
func (r *RSA) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rsa",
		GroupID: "encrypt",
		Short:   "RSA public key and private key tools",
		Long: `Generate RSA public and private key files.

site: https://gorm.io/gen`,
		Example: `# Generate RSA public and private key files with default settings
command rsa

# Generate RSA keys with specific format, encoding, bits, output directory
command rsa --format PKCS1 -e DER -b 4096 -o ./keys

# Generate RSA keys with PEM encoding and 2048 bits
command rsa -e PEM -b 2048`,
		Args: cobra.MaximumNArgs(0),
		Run:  r.run,
	}

	// Setup flags
	r.flags(cmd)
	return cmd
}

// flags setup flags for the RSA command.
func (r *RSA) flags(c *cobra.Command) {
	c.Flags().StringVar(&r.format, "format", "PKCS8", "Specify the key format: PKCS1 or PKCS8")
	c.Flags().StringVarP(&r.encoding, "encoding", "e", "PEM", "Specify the key encoding: PEM or DER")
	c.Flags().IntVarP(&r.bits, "bits", "b", 2048, "Specify the key length in bits")
	c.Flags().StringVarP(&r.outDir, "out", "o", "./out", "Specify the output directory for the generated key files")
}

// run executes the RSA command logic.
func (r *RSA) run(_ *cobra.Command, _ []string) {
	if err := r.validate(); err != nil {
		color.Red("Error: %v \n\n", err)
		return
	}
	if err := r.exec(); err != nil {
		color.Red("Error: %v \n\n", err)
		return
	}

	color.Green("RSA keys generated successfully!\n\n")
}

// exec executes the RSA key generation logic.
func (r *RSA) exec() error {
	// Ensure output directory exists
	if err := os.MkdirAll(r.outDir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	// Generate RSA private key
	pubKey, err := r.private()
	if err != nil {
		return err
	}

	// Generate RSA public key
	if err := r.public(pubKey); err != nil {
		return err
	}
	return nil
}

// public generates an RSA public key and writes it to a file.
func (r *RSA) public(pubKey *rsa.PublicKey) (err error) {
	var pubBytes, pubOut []byte
	var pubBlockType string

	// Marshal public key based on format
	switch r.format {
	case "PKCS1":
		pubBytes = x509.MarshalPKCS1PublicKey(pubKey)
		pubBlockType = "RSA PUBLIC KEY"
	case "PKCS8":
		pubBytes, err = x509.MarshalPKIXPublicKey(pubKey)
		if err != nil {
			return fmt.Errorf("failed to marshal PKCS8 public key: %w", err)
		}
		pubBlockType = "PUBLIC KEY"
	default:
		return fmt.Errorf("unsupported format: %s", r.format)
	}

	// Encode public key based on encoding
	if r.encoding == "PEM" {
		pubOut = pem.EncodeToMemory(&pem.Block{Type: pubBlockType, Bytes: pubBytes})
	} else {
		pubOut = pubBytes
	}

	// Write keys to files
	pubPath := filepath.Join(r.outDir, "public."+ext(r.encoding))

	if err := os.WriteFile(pubPath, pubOut, 0644); err != nil {
		return fmt.Errorf("write public: %w", err)
	}
	return nil
}

// private generates an RSA private key and writes it to a file.
func (r *RSA) private() (*rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, r.bits)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA private key: %w", err)
	}

	var privBytes, privOut []byte
	var privBlockType string

	// Marshal private key based on format
	switch r.format {
	case "PKCS1":
		privBytes = x509.MarshalPKCS1PrivateKey(privateKey)
		privBlockType = "RSA PRIVATE KEY"
	case "PKCS8":
		privBytes, err = x509.MarshalPKCS8PrivateKey(privateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal PKCS8 private key: %w", err)
		}
		privBlockType = "PRIVATE KEY"
	default:
		return nil, fmt.Errorf("unsupported format: %s", r.format)
	}

	// Encode private key based on encoding
	switch r.encoding {
	case "DER":
		privOut = privBytes
	case "PEM":
		privOut = pem.EncodeToMemory(&pem.Block{Type: privBlockType, Bytes: privBytes})
	default:
		return nil, fmt.Errorf("unsupported encoding: %s", r.encoding)
	}

	// Write private key to file
	privPath := filepath.Join(r.outDir, "private."+ext(r.encoding))
	if err := os.WriteFile(privPath, privOut, 0600); err != nil {
		return nil, fmt.Errorf("write private: %w", err)
	}

	return &privateKey.PublicKey, nil
}

// ext returns the file extension based on the encoding type.
func ext(encoding string) string {
	if encoding == "PEM" {
		return "pem"
	}
	return "der"
}

// validate checks if the provided flags are valid.
func (r *RSA) validate() error {
	switch r.encoding {
	case "PEM", "DER":
	default:
		return fmt.Errorf("invalid encoding: %s, must be PEM or DER", r.encoding)
	}

	switch r.bits {
	case 1024, 2048, 3072, 4096:
	default:
		return fmt.Errorf("invalid bits: %d, must be one of 1024, 2048, 3072, 4096", r.bits)
	}

	switch r.format {
	case "PKCS1", "PKCS8":
	default:
		return fmt.Errorf("invalid format: %s, must be PKCS1 or PKCS8", r.format)
	}

	return nil
}

var _ cmd.ICommand = (*RSA)(nil)
