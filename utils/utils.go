package utils

// IsKeyInMap returns true if the given `key` is the map, else false.
func IsKeyInMap(key string, lookupMap map[string]string) bool {
	for mapKey := range lookupMap {
		if mapKey == key {
			return true
		}
	}
	return false
}
