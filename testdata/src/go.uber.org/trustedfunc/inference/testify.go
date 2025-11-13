package inference

import "stubs/github.com/stretchr/testify/suite"

var dummy bool

type myErr struct{}

func (myErr) Error() string { return "myErr message" }

type S struct {
	f *int
}

type SSuite struct {
	suite.Suite
	S *S
}

func NewS() (*S, error) {
	if dummy {
		return &S{}, nil
	}
	return nil, myErr{}
}

func (s *SSuite) SetupTest() {
	var err error
	s.S, err = NewS()
	s.NoError(err)
	print(s.S.f) // safe
}

func (s *SSuite) TestField() {
	print(s.S.f) // safe since SetupTest is called before this
}
