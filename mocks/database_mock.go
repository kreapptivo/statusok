package mocks

import (
	"statusok/database"

	"github.com/stretchr/testify/mock"
)

/*
type Database interface {
	Initialize() error
	GetDatabaseName() string
	AddRequestInfo(requestInfo RequestInfo) error
	AddErrorInfo(errorInfo ErrorInfo) error
}
*/

type MockedDatabase struct {
	mock.Mock
}

func (m *MockedDatabase) Initialize() error {
	args := m.Called()
	// return nil
	if len(args) > 0 {
		return args.Get(0).(error)
	}
	return nil
}

func (m *MockedDatabase) GetDatabaseName() string {
	args := m.Called()

	if len(args) > 0 {
		return args.Get(0).(string)
	}
	return "Mocked Database"
}

func (m *MockedDatabase) AddRequestInfo(requestInfo database.RequestInfo) error {
	args := m.Called()

	if len(args) > 0 {
		return args.Get(0).(error)
	}
	return nil
}

func (m *MockedDatabase) AddErrorInfo(errorInfo database.ErrorInfo) error {
	args := m.Called()

	if len(args) > 0 {
		return args.Get(0).(error)
	}
	return nil
}
