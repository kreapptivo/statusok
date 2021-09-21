package database_test

import (
	"errors"
	"fmt"
	"statusok/database"
	"statusok/mocks"
	"statusok/model"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitialize(t *testing.T) {
	ids := make(map[int]int64)
	ids[1] = 10
	ids[2] = 2

	database.Initialize(ids, 10, 10)

	assert.Equal(t, 10, database.MinResponseCount, "MinResponseCount not correct")
	assert.Equal(t, 10, database.ErrorCount, "ErrorCount not correct")
	t.Cleanup(func() {
		database.Initialize(make(map[int]int64), 0, 0)
	})
}

func TestCountResponsesInQueue(t *testing.T) {
	const requestId = 1
	ids := make(map[int]int64)
	ids[requestId] = 10
	ids[2] = 2
	const minRequestCount = 3
	const expectation = 2

	database.Initialize(ids, minRequestCount, 10)

	database.AddResponseTimeToRequest(requestId, 10)
	database.AddResponseTimeToRequest(requestId, 10)
	result := database.CountResponsesInQueue(requestId)

	assert.Equal(t, expectation, result)
	t.Cleanup(func() {
		database.Initialize(make(map[int]int64), 0, 0)
	})
}

func TestUpdateGetResponseQueue(t *testing.T) {
	const requestId = 1
	// ids := make(map[int]int64)

	empty := database.GetResponseQueue(requestId)

	assert.Empty(t, empty)

	queue := []int64{1, 2, 3}
	database.UpdateResponseQueue(requestId, queue)

	result := database.GetResponseQueue(requestId)

	assert.Equal(t, queue, result)
	t.Cleanup(func() {
		database.Initialize(make(map[int]int64), 0, 0)
	})
}

func TestMeanResponseCalculation(t *testing.T) {
	const requestId = 1
	ids := make(map[int]int64)
	ids[requestId] = 10
	ids[2] = 2

	t.Run("when minRequestCount = 3 and responses =3", func(t *testing.T) {
		const minRequestCount = 3
		const expectation = int64(10)
		database.Initialize(ids, minRequestCount, 10)

		database.AddResponseTimeToRequest(requestId, 10)
		database.AddResponseTimeToRequest(requestId, 15)
		database.AddResponseTimeToRequest(requestId, 5)
		result, err := database.GetMeanResponseTimeOfUrl(requestId)

		assert.Equal(t, expectation, result)
		assert.Nil(t, err)
	})

	t.Run("when not yet enough requests", func(t *testing.T) {
		const minRequestCount = 2
		database.Initialize(ids, minRequestCount, 10)
		database.AddResponseTimeToRequest(requestId, 10)

		result, err := database.GetMeanResponseTimeOfUrl(requestId)
		assert.Error(t, fmt.Errorf("The number of requests 1 has not been reached the minResponseCount %d yet.", minRequestCount), err)
		assert.Empty(t, result)
	})
	t.Cleanup(func() {
		database.Initialize(make(map[int]int64), 0, 0)
	})
}

func TestMedianResponseCalculation(t *testing.T) {
	const requestId = 1
	ids := make(map[int]int64)
	ids[requestId] = 10
	ids[2] = 2

	t.Cleanup(func() {
		database.Initialize(make(map[int]int64), 0, 0)
	})

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

	t.Run("when number of minRequests is 1", func(t *testing.T) {
		const minRequestCount = 1
		const expectation = int64(3)
		database.Initialize(ids, minRequestCount, 10)

		database.AddResponseTimeToRequest(1, 3)
		result, err := database.GetMedianResponseTimeOfUrl(1)

		assert.Equal(t, expectation, result)
		assert.Nil(t, err)
	})
}

func TestAddRequestAndErrorInfo(t *testing.T) {
	const minRequestCount = 2
	const requestId = 1
	ids := make(map[int]int64)
	ids[requestId] = 10
	ids[2] = 2

	t.Cleanup(func() {
		database.Initialize(make(map[int]int64), 0, 0)
	})

	database.Initialize(ids, minRequestCount, 10)

	errorInfo := model.ErrorInfo{
		Id:           requestId,
		Url:          "http://test.com",
		RequestType:  "GET",
		ResponseCode: 0,
		ResponseBody: "test response",
		Reason:       errors.New("test error"),
		OtherInfo:    "test other info",
	}

	database.AddErrorInfo(errorInfo)

	database.AddRequestInfo(model.RequestInfo{
		Id:                   requestId,
		Url:                  "http://test.com",
		RequestType:          "GET",
		ResponseCode:         200,
		ResponseTimeMs:       20,
		ExpectedResponseTime: 200,
	})

	database.AddRequestInfo(model.RequestInfo{
		Id:                   requestId,
		Url:                  "http://test.com",
		RequestType:          "GET",
		ResponseCode:         200,
		ResponseTimeMs:       10,
		ExpectedResponseTime: 200,
	})

	result, err := database.GetMeanResponseTimeOfUrl(requestId)

	assert.Equal(t, int64(15), result)
	assert.Nil(t, err)
}

func TestNotify(t *testing.T) {
	const minRequestCount = 2
	const requestId = 1
	ids := make(map[int]int64)
	ids[requestId] = 10
	ids[2] = 2

	t.Cleanup(func() {
		database.Initialize(make(map[int]int64), 0, 0)
	})

	database.Initialize(ids, minRequestCount, 10)

	database.AddRequestInfo(model.RequestInfo{
		Id:                   requestId,
		Url:                  "http://test.com",
		RequestType:          "GET",
		ResponseCode:         200,
		ResponseTimeMs:       200,
		ExpectedResponseTime: 20,
	})
	assert.NotEmpty(t, database.GetResponseQueue(requestId))

	database.AddRequestInfo(model.RequestInfo{
		Id:                   requestId,
		Url:                  "http://test.com",
		RequestType:          "GET",
		ResponseCode:         200,
		ResponseTimeMs:       100,
		ExpectedResponseTime: 20,
	})

	assert.Empty(t, database.GetResponseQueue(requestId))
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
	t.Cleanup(func() {
		database.ResetDatabases()
	})
	const expected = 0

	ids := make(map[int]int64)
	ids[1] = 10
	ids[2] = 2

	t.Cleanup(func() {
		database.Initialize(make(map[int]int64), 0, 0)
	})

	database.Initialize(ids, 1, 10)

	influxDb := database.InfluxDb{}

	err := database.AddNew(&influxDb)

	assert.Equal(t, database.ConfiguredDatabases(), expected, "Empty Database should not be added to list")
	assert.Nil(t, err)
}

func TestAddHappyPathDatabase(t *testing.T) {
	const expected = 1

	ids := make(map[int]int64)
	ids[1] = 10
	ids[2] = 2

	t.Cleanup(func() {
		database.Initialize(make(map[int]int64), 0, 0)
	})

	database.Initialize(ids, 1, 10)

	mockedDb := new(mocks.MockedDatabase) // create the mock

	mockedDb.On("Initialize").Return().Once() // mock the expectation
	mockedDb.On("AddRequestInfo").Return().Once()
	mockedDb.On("AddErrorInfo").Return().Once()
	mockedDb.On("GetDatabaseName").Return().Once()
	mockedDb.On("IsEmpty").Return(false).Once()

	var db database.Database = mockedDb
	t.Cleanup(func() {
		database.ResetDatabases()
	})
	err := database.AddNew(db)

	assert.Nil(t, err)
	assert.Equal(t, database.ConfiguredDatabases(), expected, "Not able to add database to list")
}

func TestAddDatabaseErrorInit(t *testing.T) {
	ids := make(map[int]int64)
	ids[1] = 10
	ids[2] = 2

	database.Initialize(ids, 1, 10)
	t.Cleanup(func() {
		database.Initialize(make(map[int]int64), 0, 0)
	})
	mockedDb := new(mocks.MockedDatabase) // create the mock

	mockedDb.On("Initialize").Return(errors.New("test")).Once() // mock the expectation
	mockedDb.On("AddRequestInfo").Return().Once()
	mockedDb.On("AddErrorInfo").Return().Once()
	mockedDb.On("GetDatabaseName").Return().Once()
	mockedDb.On("IsEmpty").Return(false).Once()

	var db database.Database = mockedDb
	t.Cleanup(func() {
		database.ResetDatabases()
	})
	err := database.AddNew(db)

	assert.EqualError(t, err, "Failed to Intialize Database Mocked Database, err: test")
	assert.Zero(t, database.ConfiguredDatabases())
}

func TestAddDatabaseErrorRequestInfoWrite(t *testing.T) {
	ids := make(map[int]int64)
	ids[1] = 10
	ids[2] = 2

	t.Cleanup(func() {
		database.Initialize(make(map[int]int64), 0, 0)
	})

	database.Initialize(ids, 1, 10)

	mockedDb := new(mocks.MockedDatabase) // create the mock

	mockedDb.On("Initialize").Return().Once()
	mockedDb.On("AddRequestInfo").Return(errors.New("test")).Once()
	mockedDb.On("AddErrorInfo").Return().Once()
	mockedDb.On("GetDatabaseName").Return().Times(3)
	mockedDb.On("IsEmpty").Return(false).Once()

	var db database.Database = mockedDb
	t.Cleanup(func() {
		database.ResetDatabases()
	})
	err := database.AddNew(db)

	assert.EqualError(t, err, "Failed to access Database Mocked Database, err: InfluxDB: Failed to insert Request data to database Mocked Database, error: test. Please check whether database is installed properly!")
	assert.Zero(t, database.ConfiguredDatabases())
}

func TestAddDatabaseErrorRequestErrorWrite(t *testing.T) {
	ids := make(map[int]int64)
	ids[1] = 10
	ids[2] = 2

	database.Initialize(ids, 1, 10)

	t.Cleanup(func() {
		database.Initialize(make(map[int]int64), 0, 0)
	})

	mockedDb := new(mocks.MockedDatabase) // create the mock

	mockedDb.On("Initialize").Return().Once()
	mockedDb.On("AddRequestInfo").Return().Once()
	mockedDb.On("AddErrorInfo").Return(errors.New("test")).Once()
	mockedDb.On("GetDatabaseName").Return().Times(3)
	mockedDb.On("IsEmpty").Return(false).Once()

	var db database.Database = mockedDb
	t.Cleanup(func() {
		database.ResetDatabases()
	})
	err := database.AddNew(db)

	assert.EqualError(t, err, "Failed to access Database Mocked Database, err: InfluxDB: Failed to insert Error data to database Mocked Database, error: test. Please check whether database is installed properly!")
	assert.Zero(t, database.ConfiguredDatabases())
}

func TestParseDBConfigEmpty(t *testing.T) {
	err := database.ParseDBConfig(database.DatabaseTypes{})

	assert.Nil(t, err)
}

func TestParseDBConfigEmptyInfluxdb(t *testing.T) {
	err := database.ParseDBConfig(database.DatabaseTypes{InfluxDb: database.InfluxDb{}})

	assert.Nil(t, err)
}
