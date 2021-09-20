package database_test

import (
	"errors"
	"fmt"
	"statusok/database"
	"statusok/mocks"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitialize(t *testing.T) {
	ids := make(map[int]int64)
	ids[1] = 10
	ids[2] = 2

	database.Initialize(ids, 10, 10)

	// assert.EqualValues(t, len(ids), len(database.GetResponseQueue()), "Ids not initialized")

	assert.Equal(t, 10, database.MinResponseCount, "MinResponseCount not correct")
	assert.Equal(t, 10, database.ErrorCount, "ErrorCount not correct")
}

func TestMeanResponseCalculation(t *testing.T) {
	ids := make(map[int]int64)
	ids[1] = 10
	ids[2] = 2

	database.Initialize(ids, 3, 10)

	database.AddResponseTimeToRequest(1, 10)
	database.AddResponseTimeToRequest(1, 15)
	database.AddResponseTimeToRequest(1, 5)
	result, err := database.GetMeanResponseTimeOfUrl(1)

	assert.Equal(t, int64(10), result)
	assert.Nil(t, err)
}

func TestMedianResponseCalculation(t *testing.T) {
	const requestId = 1
	ids := make(map[int]int64)
	ids[requestId] = 10
	ids[2] = 2

	t.Run("when minRequestCount = 3 and reponses are 4, use only 3 latest", func(t *testing.T) {
		const minRequestCount = 3
		const expectation = int64(8)
		database.Initialize(ids, minRequestCount, 10)

		database.AddResponseTimeToRequest(requestId, 10)
		database.AddResponseTimeToRequest(requestId, 3)
		database.AddResponseTimeToRequest(requestId, 9)
		database.AddResponseTimeToRequest(requestId, 8)
		result, err := database.GetMedianResponseTimeOfUrl(requestId)

		assert.Equal(t, expectation, result)
		assert.Nil(t, err)
	})

	t.Run("when number of requests is odd", func(t *testing.T) {
		const minRequestCount = 3
		const expectation = int64(4)
		database.Initialize(ids, minRequestCount, 10)

		database.AddResponseTimeToRequest(1, 3)
		database.AddResponseTimeToRequest(1, 4)
		database.AddResponseTimeToRequest(1, 8)
		result, err := database.GetMedianResponseTimeOfUrl(1)

		assert.Equal(t, expectation, result)
		assert.Nil(t, err)
	})

	t.Run("when number of requests is even", func(t *testing.T) {
		const minRequestCount = 4
		const expectation = int64(6)
		database.Initialize(ids, minRequestCount, 10)

		database.AddResponseTimeToRequest(1, 10)
		database.AddResponseTimeToRequest(1, 3)
		database.AddResponseTimeToRequest(1, 4)
		database.AddResponseTimeToRequest(1, 8)
		result, err := database.GetMedianResponseTimeOfUrl(1)

		assert.Equal(t, expectation, result)
		assert.Nil(t, err)
	})

	t.Run("when not yet enough requests", func(t *testing.T) {
		const minRequestCount = 2
		database.Initialize(ids, minRequestCount, 10)
		database.AddResponseTimeToRequest(requestId, 10)

		result, err := database.GetMedianResponseTimeOfUrl(requestId)
		assert.Error(t, fmt.Errorf("The number of requests 1 has not been reached the minResponseCount %d yet.", minRequestCount), err)
		assert.Empty(t, result)
	})
}

func TestAddRequestAndErrorInfo(t *testing.T) {
	const minRequestCount = 2
	const requestId = 1
	ids := make(map[int]int64)
	ids[requestId] = 10
	ids[2] = 2

	database.Initialize(ids, minRequestCount, 10)

	errorInfo := database.ErrorInfo{requestId, "http://test.com", "GET", 0, "test response", errors.New("test error"), "test other info"}

	database.AddErrorInfo(errorInfo)

	database.AddRequestInfo(database.RequestInfo{requestId, "http://test.com", "GET", 200, 20, 200})
	database.AddRequestInfo(database.RequestInfo{requestId, "http://test.com", "GET", 200, 10, 200})

	result, err := database.GetMeanResponseTimeOfUrl(requestId)

	assert.Equal(t, int64(15), result)
	assert.Nil(t, err)
}

func TestClearQueue(t *testing.T) {
	ids := make(map[int]int64)
	ids[1] = 10
	ids[2] = 2

	database.Initialize(ids, 1, 10)

	database.AddResponseTimeToRequest(1, 10)

	assert.NotEmpty(t, database.GetResponseQueue(1))
	database.ClearQueue(1)
	assert.Empty(t, database.GetResponseQueue(1), "ClearQueue Function is not working")
}

func TestAddEmptyDatabase(t *testing.T) {
	const expected = 0

	ids := make(map[int]int64)
	ids[1] = 10
	ids[2] = 2

	database.Initialize(ids, 1, 10)

	influxDb := database.InfluxDb{}

	err := database.AddNew(influxDb)

	assert.Equal(t, database.ConfiguredDatabases(), expected, "Empty Database should not be added to list")
	assert.Nil(t, err)
}

func TestAddValidDatabase(t *testing.T) {
	const expected = 1

	ids := make(map[int]int64)
	ids[1] = 10
	ids[2] = 2

	database.Initialize(ids, 1, 10)

	mockedDb := new(mocks.MockedDatabase)     // create the mock
	mockedDb.On("Initialize").Return().Once() // mock the expectation
	mockedDb.On("AddRequestInfo").Return().Once()
	mockedDb.On("AddErrorInfo").Return().Once()
	mockedDb.On("GetDatabaseName").Return().Once()

	var db database.Database = mockedDb

	err := database.AddNew(db)

	assert.Nil(t, err)
	assert.Equal(t, database.ConfiguredDatabases(), expected, "Not able to add database to list")
}
