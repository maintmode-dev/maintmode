package xlo

func ToPtrOrNil[T comparable](v T) *T {
	var zero T

	if zero == v {
		return nil
	}

	return &v
}
