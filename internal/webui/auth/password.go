package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	passwordAlgorithm  = "pbkdf2_sha256"
	passwordIterations = 210_000
	passwordSaltBytes  = 16
	passwordHashBytes  = 32
)

func ValidateSettings(settings Settings) error {
	if settings.PasswordMinLength < 1 || settings.PasswordMinLength > maxPasswordLength {
		return fmt.Errorf("密码最小长度需为 1-%d 个字符", maxPasswordLength)
	}
	if !settings.PasswordRequireLetters && !settings.PasswordRequireDigits && !settings.PasswordRequireSymbols {
		return fmt.Errorf("至少需要启用一种密码字符类型")
	}
	if settings.SessionTTLSeconds < int64(minSessionTTL/time.Second) || settings.SessionTTLSeconds > int64(maxSessionTTL/time.Second) {
		return fmt.Errorf("session TTL must be between 15 minutes and 90 days")
	}
	if settings.IdleTimeoutSeconds < 0 || settings.IdleTimeoutSeconds > int64(maxIdleTimeout/time.Second) {
		return fmt.Errorf("idle timeout must be disabled or no more than 30 days")
	}
	return nil
}

func ValidatePassword(password string, settings Settings) error {
	if err := ValidateSettings(settings); err != nil {
		return err
	}
	if len(password) < settings.PasswordMinLength || len(password) > maxPasswordLength {
		return fmt.Errorf("密码长度需为 %d-%d 个字符", settings.PasswordMinLength, maxPasswordLength)
	}
	var hasLetter, hasDigit, hasSymbol bool
	for _, char := range []byte(password) {
		switch {
		case settings.PasswordRequireLetters && isASCIILetter(char):
			hasLetter = true
		case settings.PasswordRequireDigits && isASCIIDigit(char):
			hasDigit = true
		case settings.PasswordRequireSymbols && isASCIISymbol(char):
			hasSymbol = true
		default:
			return fmt.Errorf("密码只能包含%s，不能包含空格、中文或未启用的字符类型", enabledPasswordTypesText(settings))
		}
	}
	if settings.PasswordRequireLetters && !hasLetter {
		return fmt.Errorf("密码需包含英文字母")
	}
	if settings.PasswordRequireDigits && !hasDigit {
		return fmt.Errorf("密码需包含数字")
	}
	if settings.PasswordRequireSymbols && !hasSymbol {
		return fmt.Errorf("密码需包含符号")
	}
	return nil
}

func HashPassword(password string, settings Settings) (string, error) {
	if err := ValidatePassword(password, settings); err != nil {
		return "", err
	}
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, passwordHashBytes)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return strings.Join([]string{
		passwordAlgorithm,
		strconv.Itoa(passwordIterations),
		base64.RawURLEncoding.EncodeToString(salt),
		base64.RawURLEncoding.EncodeToString(hash),
	}, "$"), nil
}

func VerifyPassword(password, encoded string) (bool, error) {
	if len(password) > maxPasswordLength {
		return false, nil
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != passwordAlgorithm {
		return false, fmt.Errorf("unsupported password hash format")
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 100_000 || iterations > 2_000_000 {
		return false, fmt.Errorf("invalid password hash iterations")
	}
	salt, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(salt) < 16 || len(salt) > 64 {
		return false, fmt.Errorf("invalid password hash salt")
	}
	expected, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil || len(expected) != passwordHashBytes {
		return false, fmt.Errorf("invalid password hash")
	}
	actual, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(expected))
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func isASCIILetter(char byte) bool {
	return char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z'
}

func isASCIIDigit(char byte) bool {
	return char >= '0' && char <= '9'
}

func isASCIISymbol(char byte) bool {
	return char >= '!' && char <= '~' && !isASCIILetter(char) && !isASCIIDigit(char)
}

func enabledPasswordTypesText(settings Settings) string {
	types := make([]string, 0, 3)
	if settings.PasswordRequireLetters {
		types = append(types, "英文字母")
	}
	if settings.PasswordRequireDigits {
		types = append(types, "数字")
	}
	if settings.PasswordRequireSymbols {
		types = append(types, "符号")
	}
	switch len(types) {
	case 0:
		return "英文字母、数字和符号"
	case 1:
		return types[0]
	case 2:
		return types[0] + "和" + types[1]
	default:
		return strings.Join(types[:len(types)-1], "、") + "和" + types[len(types)-1]
	}
}
