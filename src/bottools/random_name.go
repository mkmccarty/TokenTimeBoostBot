package bottools

import (
	"fmt"
	"math/rand/v2"
	"strings"
)

var randomNameLeft = []string{
	"brisk", "calm", "clever", "cosmic", "crisp", "daring", "gentle", "lucky", "mellow", "nimble",
	"quiet", "rapid", "savvy", "steady", "swift", "vivid", "witty", "bold", "bright", "chill",
}

var randomNameRight = []string{
	"acorn", "anchor", "aster", "beacon", "bison", "comet", "drifter", "falcon", "harbor", "meadow",
	"nebula", "otter", "ranger", "rocket", "sailor", "sprout", "thunder", "valley", "voyager", "zephyr",
}

// GetRandomName returns a docker-style random name.
func GetRandomName(_ int) string {
	left := randomNameLeft[rand.IntN(len(randomNameLeft))]
	right := randomNameRight[rand.IntN(len(randomNameRight))]
	return fmt.Sprintf("%s_%s", left, right)
}

// IsRandomName returns true if name matches the generated docker-style random name format.
func IsRandomName(name string) bool {
	left, right, ok := strings.Cut(name, "_")
	if !ok || left == "" || right == "" {
		return false
	}

	leftValid := false
	for _, part := range randomNameLeft {
		if left == part {
			leftValid = true
			break
		}
	}
	if !leftValid {
		return false
	}

	for _, part := range randomNameRight {
		if right == part {
			return true
		}
	}

	return false
}
