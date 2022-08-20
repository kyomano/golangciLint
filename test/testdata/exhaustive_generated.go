//golangcitest:args -Eexhaustive
//golangcitest:expected_exitcode 0
package testdata

// Code generated by some program. DO NOT EDIT.

// Should not report missing cases in the switch statement below, because this
// is a generated file as indicated by the above comment
// (golang.org/s/generatedcode), and check-generated setting is false.

type Direction int

const (
	North Direction = iota
	East
	South
	West
)

func processDirectionGenerated(d Direction) {
	switch d {
	case North, South:
	}
}
