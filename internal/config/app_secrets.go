package config

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"

	"github.com/spf13/viper"
)

var secretRegexp = regexp.MustCompile(`^<secret:(.+)>$`)

type secretStore map[string]string

func readSecrets(filePath string) (secretStore, error) {
	reader := viper.New()
	reader.SetConfigFile(filePath)

	if err := reader.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filePath, err)
	}

	secrets := make(secretStore)
	if err := reader.Unmarshal(&secrets); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %s: %w", filePath, err)
	}

	return secrets, nil
}

type secretResolver struct {
	secrets secretStore
}

//nolint:gocognit,gocyclo
func (r secretResolver) resolveValue(value reflect.Value) error {
	if !value.IsValid() {
		return nil
	}

	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		if value.IsNil() {
			return nil
		}
		return r.resolveValue(value.Elem())
	case reflect.Struct:
		var errs error
		for i := range value.NumField() {
			pkgPath := value.Type().Field(i).PkgPath
			if pkgPath != "" {
				continue
			}

			errs = errors.Join(errs, r.resolveValue(value.Field(i)))
		}
		return errs
	case reflect.Map:
		var errs error
		for _, key := range value.MapKeys() {
			mapValue := value.MapIndex(key)
			mapValueCopy := reflect.New(mapValue.Type()).Elem()
			mapValueCopy.Set(mapValue)

			if err := r.resolveValue(mapValueCopy); err != nil {
				errs = errors.Join(errs, err)
				continue
			}

			value.SetMapIndex(key, mapValueCopy)
		}
		return errs
	case reflect.Slice, reflect.Array:
		var errs error
		for i := range value.Len() {
			errs = errors.Join(errs, r.resolveValue(value.Index(i)))
		}

		return errs
	case reflect.String:
		key := value.String()
		if !secretRegexp.MatchString(key) {
			return nil
		}

		resolved, err := r.resolveSecret(key)
		if err != nil {
			return err
		}
		if resolved != value.String() && value.CanSet() {
			value.SetString(resolved)
		}
	default:
		return nil
	}

	return nil
}

func (r secretResolver) resolveSecret(key string) (string, error) {
	parts := secretRegexp.FindStringSubmatch(key)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid secret key format '%s'. format: %s", key, secretRegexp.String())
	}

	secretKey := parts[1]
	secret, ok := r.secrets[secretKey]
	if !ok {
		return "", fmt.Errorf("secret not found: %s", secretKey)
	}

	return secret, nil
}
