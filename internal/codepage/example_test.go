package codepage_test

import (
	"fmt"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/codepage"
)

func Example() {
	fmt.Println(codepage.DecodeISO6937([]byte("Caf\xC2e")))
	fmt.Printf("% X\n", codepage.EncodeISO6937("Café"))
	fmt.Printf("% X\n", codepage.CP850.Encode("Météo"))
	// Output:
	// Café
	// 43 61 66 C2 65
	// 4D 82 74 82 6F
}
