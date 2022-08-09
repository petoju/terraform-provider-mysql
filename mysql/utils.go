package mysql

import (
	"crypto/sha256"
	"fmt"
	"strconv"
)

func hashSum(contents interface{}) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(contents.(string))))
}

func isNumeric(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}
