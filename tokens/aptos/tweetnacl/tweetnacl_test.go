package tweetnacl

import (
	"bytes"
	"strings"
	"testing"
)

func verify(t *testing.T, description string, expected, result []byte, err error) {
	padding := strings.Repeat(" ", len(description))

	if err != nil {
		t.Errorf("\n%v", err)
		return
	}

	if result == nil {
		t.Errorf("\n%s - expected %x\n%s - got:     nil", description, expected, padding)
		return
	}

	if !bytes.Equal(result, expected) {
		t.Errorf("\n%s - expected %x\n%s - got:     %x", description, expected, padding, result)
		return
	}
}

func verifyErr(t *testing.T, description string, result []byte, err error) {
	if err == nil {
		t.Errorf("\nExpected error (%s)", description)
	}

	if result != nil {
		t.Errorf("\nExpected error (%s)", description)
	}
}
