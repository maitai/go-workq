package workq

import (
	"bytes"
	"errors"
	"net"
	"testing"
	"time"
)

func TestConnectAndClose(t *testing.T) {
	addr := "localhost:9944"
	_, err := Connect(addr)
	if err == nil {
		t.Fatalf("Unexpected connect")
	}

	server, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("Unable to start test server, err=%s", err)
	}
	defer server.Close()

	client, err := Connect(addr)
	if err != nil {
		t.Fatalf("Unable to connect, err=%s", err)
	}

	err = client.Close()
	if err != nil {
		t.Fatalf("Unable to close, err=%s", err)
	}

	err = client.Close()
	if err == nil {
		t.Fatal("Expected error on double close")
	}
}

func TestAdd(t *testing.T) {
	conn := &TestConn{
		rdr: bytes.NewBuffer([]byte("+OK\r\n")),
		wrt: bytes.NewBuffer([]byte("")),
	}
	client := NewClient(conn)
	j := &BgJob{
		ID:      "6ba7b810-9dad-11d1-80b4-00c04fd430c4",
		Name:    "j1",
		TTR:     60,
		TTL:     60000,
		Payload: []byte("a"),
	}
	err := client.Add(j)
	if err != nil {
		t.Fatalf("Response mismatch, err=%s", err)
	}

	expWrite := []byte(
		"add 6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 60 60000 1\r\na\r\n",
	)
	if !bytes.Equal(expWrite, conn.wrt.Bytes()) {
		t.Fatalf("Write mismatch, act=%q", conn.wrt.Bytes())
	}
}

func TestAddOptionalFlags(t *testing.T) {
	tests := []struct {
		job      *BgJob
		expWrite []byte
	}{
		{
			job: &BgJob{
				ID:       "6ba7b810-9dad-11d1-80b4-00c04fd430c4",
				Name:     "j1",
				TTR:      1,
				TTL:      2,
				Payload:  []byte(""),
				Priority: 100,
			},
			expWrite: []byte(
				"add 6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 1 2 0 -priority=100\r\n\r\n",
			),
		},
		{
			job: &BgJob{
				ID:          "6ba7b810-9dad-11d1-80b4-00c04fd430c4",
				Name:        "j1",
				TTR:         1,
				TTL:         2,
				Payload:     []byte(""),
				MaxAttempts: 3,
			},
			expWrite: []byte(
				"add 6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 1 2 0 -max-attempts=3\r\n\r\n",
			),
		},
		{
			job: &BgJob{
				ID:       "6ba7b810-9dad-11d1-80b4-00c04fd430c4",
				Name:     "j1",
				TTR:      1,
				TTL:      2,
				Payload:  []byte(""),
				MaxFails: 3,
			},
			expWrite: []byte(
				"add 6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 1 2 0 -max-fails=3\r\n\r\n",
			),
		},
		{
			job: &BgJob{
				ID:          "6ba7b810-9dad-11d1-80b4-00c04fd430c4",
				Name:        "j1",
				TTR:         1,
				TTL:         2,
				Payload:     []byte(""),
				Priority:    1,
				MaxAttempts: 3,
				MaxFails:    1,
			},
			expWrite: []byte(
				"add 6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 1 2 0 -priority=1 -max-attempts=3 -max-fails=1\r\n\r\n",
			),
		},
	}

	for _, tt := range tests {
		conn := &TestConn{
			rdr: bytes.NewBuffer([]byte("+OK\r\n")),
			wrt: bytes.NewBuffer([]byte("")),
		}
		client := NewClient(conn)
		err := client.Add(tt.job)
		if err != nil {
			t.Fatalf("Response mismatch, err=%s", err)
		}

		if !bytes.Equal(tt.expWrite, conn.wrt.Bytes()) {
			t.Fatalf("Write mismatch, act=%q", conn.wrt.Bytes())
		}
	}
}

func TestAddErrors(t *testing.T) {
	tests := []RespErrTestCase{
		{
			resp:   []byte("-CLIENT-ERROR Invalid Job ID\r\n"),
			expErr: errors.New("CLIENT-ERROR Invalid Job ID"),
		},
	}

	tests = append(tests, invalidCommonErrorTests()...)

	for _, tt := range tests {
		conn := &TestConn{
			rdr: bytes.NewBuffer(tt.resp),
			wrt: bytes.NewBuffer([]byte("")),
		}
		client := NewClient(conn)
		j := &BgJob{}
		err := client.Add(j)
		if err == nil || tt.expErr == nil || err.Error() != tt.expErr.Error() {
			t.Fatalf("Response mismatch, err=%q", err)
		}
	}
}

func TestAddBadConnError(t *testing.T) {
	conn := &TestBadWriteConn{}
	client := NewClient(conn)
	j := &BgJob{}
	err := client.Add(j)
	if _, ok := err.(*NetError); !ok {
		t.Fatalf("Error mismatch, err=%+v", err)
	}
}

func TestRun(t *testing.T) {
	conn := &TestConn{
		rdr: bytes.NewBuffer([]byte(
			"+OK 1\r\n" +
				"6ba7b810-9dad-11d1-80b4-00c04fd430c4 1 1\r\n" +
				"a\r\n",
		)),
		wrt: bytes.NewBuffer([]byte("")),
	}
	client := NewClient(conn)
	j := &FgJob{
		ID:      "6ba7b810-9dad-11d1-80b4-00c04fd430c4",
		Name:    "j1",
		TTR:     5000,
		Timeout: 1000,
		Payload: []byte("a"),
	}
	result, err := client.Run(j)
	if err != nil {
		t.Fatalf("Response mismatch, err=%s", err)
	}

	if !result.Success {
		t.Fatalf("Success mismatch")
	}

	if !bytes.Equal([]byte("a"), result.Result) {
		t.Fatalf("Result mismatch")
	}

	expWrite := []byte(
		"run 6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 5000 1000 1\r\na\r\n",
	)
	if !bytes.Equal(expWrite, conn.wrt.Bytes()) {
		t.Fatalf("Write mismatch, act=%s", conn.wrt.Bytes())
	}
}

func TestRunOptionalFlags(t *testing.T) {
	conn := &TestConn{
		rdr: bytes.NewBuffer([]byte(
			"+OK 1\r\n" +
				"6ba7b810-9dad-11d1-80b4-00c04fd430c4 1 1\r\n" +
				"a\r\n",
		)),
		wrt: bytes.NewBuffer([]byte("")),
	}
	client := NewClient(conn)
	j := &FgJob{
		ID:       "6ba7b810-9dad-11d1-80b4-00c04fd430c4",
		Name:     "j1",
		TTR:      5000,
		Timeout:  1000,
		Payload:  []byte("a"),
		Priority: 1,
	}
	result, err := client.Run(j)
	if err != nil {
		t.Fatalf("Response mismatch, err=%s", err)
	}

	if !result.Success {
		t.Fatalf("Success mismatch")
	}

	if !bytes.Equal([]byte("a"), result.Result) {
		t.Fatalf("Result mismatch")
	}

	expWrite := []byte(
		"run 6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 5000 1000 1 -priority=1\r\na\r\n",
	)
	if !bytes.Equal(expWrite, conn.wrt.Bytes()) {
		t.Fatalf("Write mismatch, act=%s", conn.wrt.Bytes())
	}
}

func TestRunErrors(t *testing.T) {
	tests := []RespErrTestCase{
		{
			resp:   []byte("-CLIENT-ERROR Invalid Job ID\r\n"),
			expErr: errors.New("CLIENT-ERROR Invalid Job ID"),
		},
	}
	tests = append(tests, invalidCommonErrorTests()...)
	tests = append(tests, invalidResultErrorTests()...)

	for _, tt := range tests {
		conn := &TestConn{
			rdr: bytes.NewBuffer(tt.resp),
			wrt: bytes.NewBuffer([]byte("")),
		}
		client := NewClient(conn)
		j := &FgJob{
			ID:      "6ba7b810-9dad-11d1-80b4-00c04fd430c4",
			Name:    "j1",
			TTR:     5000,
			Timeout: 1000,
			Payload: []byte("a"),
		}
		result, err := client.Run(j)
		if result != nil || err == nil || tt.expErr == nil || err.Error() != tt.expErr.Error() {
			t.Fatalf("Response mismatch, result=%v, err=%q", result, err)
		}

		expWrite := []byte(
			"run 6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 5000 1000 1\r\na\r\n",
		)
		if !bytes.Equal(expWrite, conn.wrt.Bytes()) {
			t.Fatalf("Write mismatch, act=%s", conn.wrt.Bytes())
		}
	}
}

func TestRunBadConnError(t *testing.T) {
	conn := &TestBadWriteConn{}
	client := NewClient(conn)
	j := &FgJob{}
	result, err := client.Run(j)
	if _, ok := err.(*NetError); !ok {
		t.Fatalf("Error mismatch, err=%+v", err)
	}

	if result != nil {
		t.Fatalf("Response mismatch, resp=%+v", result)
	}
}

func TestSchedule(t *testing.T) {
	conn := &TestConn{
		rdr: bytes.NewBuffer([]byte("+OK\r\n")),
		wrt: bytes.NewBuffer([]byte("")),
	}
	client := NewClient(conn)
	j := &ScheduledJob{
		ID:      "6ba7b810-9dad-11d1-80b4-00c04fd430c4",
		Name:    "j1",
		TTR:     5000,
		TTL:     60000,
		Time:    "2016-01-02T15:04:05Z",
		Payload: []byte("a"),
	}
	err := client.Schedule(j)
	if err != nil {
		t.Fatalf("Response mismatch, err=%s", err)
	}

	expWrite := []byte(
		"schedule 6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 5000 60000 2016-01-02T15:04:05Z 1\r\na\r\n",
	)
	if !bytes.Equal(expWrite, conn.wrt.Bytes()) {
		t.Fatalf("Write mismatch, act=%s", conn.wrt.Bytes())
	}
}

func TestScheduleOptionalFlags(t *testing.T) {
	tests := []struct {
		job      *ScheduledJob
		expWrite []byte
	}{
		{
			job: &ScheduledJob{
				ID:       "6ba7b810-9dad-11d1-80b4-00c04fd430c4",
				Name:     "j1",
				TTR:      1,
				TTL:      2,
				Time:     "2016-12-01T00:00:00Z",
				Payload:  []byte(""),
				Priority: 100,
			},
			expWrite: []byte(
				"schedule 6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 1 2 2016-12-01T00:00:00Z 0 -priority=100\r\n\r\n",
			),
		},
		{
			job: &ScheduledJob{
				ID:          "6ba7b810-9dad-11d1-80b4-00c04fd430c4",
				Name:        "j1",
				TTR:         1,
				TTL:         2,
				Time:        "2016-12-01T00:00:00Z",
				Payload:     []byte(""),
				MaxAttempts: 3,
			},
			expWrite: []byte(
				"schedule 6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 1 2 2016-12-01T00:00:00Z 0 -max-attempts=3\r\n\r\n",
			),
		},
		{
			job: &ScheduledJob{
				ID:       "6ba7b810-9dad-11d1-80b4-00c04fd430c4",
				Name:     "j1",
				TTR:      1,
				TTL:      2,
				Time:     "2016-12-01T00:00:00Z",
				Payload:  []byte(""),
				MaxFails: 3,
			},
			expWrite: []byte(
				"schedule 6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 1 2 2016-12-01T00:00:00Z 0 -max-fails=3\r\n\r\n",
			),
		},
		{
			job: &ScheduledJob{
				ID:          "6ba7b810-9dad-11d1-80b4-00c04fd430c4",
				Name:        "j1",
				TTR:         1,
				TTL:         2,
				Time:        "2016-12-01T00:00:00Z",
				Payload:     []byte(""),
				Priority:    1,
				MaxAttempts: 3,
				MaxFails:    1,
			},
			expWrite: []byte(
				"schedule 6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 1 2 2016-12-01T00:00:00Z 0 -priority=1 -max-attempts=3 -max-fails=1\r\n\r\n",
			),
		},
	}

	for _, tt := range tests {
		conn := &TestConn{
			rdr: bytes.NewBuffer([]byte("+OK\r\n")),
			wrt: bytes.NewBuffer([]byte("")),
		}
		client := NewClient(conn)
		err := client.Schedule(tt.job)
		if err != nil {
			t.Fatalf("Response mismatch, err=%s", err)
		}

		if !bytes.Equal(tt.expWrite, conn.wrt.Bytes()) {
			t.Fatalf("Write mismatch, act=%q", conn.wrt.Bytes())
		}
	}
}

func TestScheduleErrors(t *testing.T) {
	tests := []RespErrTestCase{
		{
			resp:   []byte("-CLIENT-ERROR Invalid Job ID\r\n"),
			expErr: errors.New("CLIENT-ERROR Invalid Job ID"),
		},
	}

	for _, tt := range tests {
		conn := &TestConn{
			rdr: bytes.NewBuffer(tt.resp),
			wrt: bytes.NewBuffer([]byte("")),
		}
		client := NewClient(conn)
		j := &ScheduledJob{}
		err := client.Schedule(j)
		if err == nil || tt.expErr == nil || err.Error() != tt.expErr.Error() {
			t.Fatalf("Response mismatch, err=%q", err)
		}
	}
}

func TestScheduleBaddConnError(t *testing.T) {
	conn := &TestBadWriteConn{}
	client := NewClient(conn)
	j := &ScheduledJob{}
	err := client.Schedule(j)
	if _, ok := err.(*NetError); !ok {
		t.Fatalf("Error mismatch, err=%+v", err)
	}
}

func TestResult(t *testing.T) {
	conn := &TestConn{
		rdr: bytes.NewBuffer([]byte(
			"+OK 1\r\n" +
				"6ba7b810-9dad-11d1-80b4-00c04fd430c4 1 1\r\n" +
				"a\r\n",
		)),
		wrt: bytes.NewBuffer([]byte("")),
	}
	client := NewClient(conn)
	result, err := client.Result("6ba7b810-9dad-11d1-80b4-00c04fd430c4", 1000)
	if err != nil {
		t.Fatalf("Response mismatch, err=%s", err)
	}

	if !result.Success {
		t.Fatalf("Success mismatch")
	}

	if !bytes.Equal([]byte("a"), result.Result) {
		t.Fatalf("Result mismatch")
	}

	expWrite := []byte(
		"result 6ba7b810-9dad-11d1-80b4-00c04fd430c4 1000\r\n",
	)
	if !bytes.Equal(expWrite, conn.wrt.Bytes()) {
		t.Fatalf("Write mismatch, act=%s", conn.wrt.Bytes())
	}
}

func TestResultTimeout(t *testing.T) {
	conn := &TestConn{
		rdr: bytes.NewBuffer([]byte(
			"-TIMED-OUT\r\n",
		)),
		wrt: bytes.NewBuffer([]byte("")),
	}
	client := NewClient(conn)
	if _, err := client.Result("6ba7b810-9dad-11d1-80b4-00c04fd430c4", 1000); err != nil {
		werr, ok := err.(*ResponseError)
		if !ok {
			t.Fatalf("Response mismatch, err=%s", err)
		}

		if werr.Code() != "TIMED-OUT" {
			t.Fatalf("Response mismatch, err=%s", err)
		}
	}

	expWrite := []byte(
		"result 6ba7b810-9dad-11d1-80b4-00c04fd430c4 1000\r\n",
	)

	if !bytes.Equal(expWrite, conn.wrt.Bytes()) {
		t.Fatalf("Write mismatch, act=%s", conn.wrt.Bytes())
	}
}

func TestResultErrors(t *testing.T) {
	var tests []RespErrTestCase
	tests = append(tests, invalidResultErrorTests()...)
	tests = append(tests, invalidCommonErrorTests()...)
	for _, tt := range tests {
		conn := &TestConn{
			rdr: bytes.NewBuffer(tt.resp),
			wrt: bytes.NewBuffer([]byte("")),
		}
		client := NewClient(conn)
		result, err := client.Result("6ba7b810-9dad-11d1-80b4-00c04fd430c4", 1000)
		if result != nil || err == nil || tt.expErr == nil || err.Error() != tt.expErr.Error() {
			t.Fatalf("Response mismatch, err=%q, expErr=%q", err, tt.expErr)
		}
	}
}

func TestResultBadConnError(t *testing.T) {
	conn := &TestBadWriteConn{}
	client := NewClient(conn)
	result, err := client.Result("6ba7b810-9dad-11d1-80b4-00c04fd430c4", 1000)
	if _, ok := err.(*NetError); !ok {
		t.Fatalf("Error mismatch, err=%+v", err)
	}

	if result != nil {
		t.Fatalf("Result mismatch, result=%+v", result)
	}
}

func TestLease(t *testing.T) {
	conn := &TestConn{
		rdr: bytes.NewBuffer([]byte(
			"+OK 1\r\n" +
				"6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 1000 1\r\n" +
				"a\r\n",
		)),
		wrt: bytes.NewBuffer([]byte("")),
	}
	client := NewClient(conn)
	j, err := client.Lease([]string{"j1"}, 1000)
	if err != nil {
		t.Fatalf("Response mismatch, err=%s", err)
	}

	if j.ID != "6ba7b810-9dad-11d1-80b4-00c04fd430c4" {
		t.Fatalf("ID mismatch")
	}

	if j.Name != "j1" {
		t.Fatalf("Name mismatch")
	}

	if j.TTR != 1000 {
		t.Fatalf("TTR mismatch")
	}

	if !bytes.Equal([]byte("a"), j.Payload) {
		t.Fatalf("Payload mismatch")
	}

	expWrite := []byte(
		"lease j1 1000\r\n",
	)
	if !bytes.Equal(expWrite, conn.wrt.Bytes()) {
		t.Fatalf("Write mismatch, act=%s", conn.wrt.Bytes())
	}
}

func TestLeaseErrors(t *testing.T) {
	tests := []RespErrTestCase{
		// Invalid reply-count
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 1\r\n" +
					"a\r\n",
			),
			expErr: ErrMalformed,
		},
		// Space after reply-count
		{
			resp: []byte(
				"+OK 1 \r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 1\r\n" +
					"a\r\n",
			),
			expErr: ErrMalformed,
		},
		// Whitespace as reply-count
		{
			resp: []byte(
				"+OK \r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 1\r\n" +
					"a\r\n",
			),
			expErr: ErrMalformed,
		},
		// Missing ID
		{
			resp: []byte(
				"+OK 1\r\n" +
					"j1 1\r\n" +
					"a\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid ID
		{
			resp: []byte(
				"+OK 1\r\n" +
					"* j1 1\r\n" +
					"a\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid name
		{
			resp: []byte(
				"+OK 1\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 * 1\r\n" +
					"a\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid size
		{
			resp: []byte(
				"+OK 1\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 *\r\n" +
					"a\r\n",
			),
			expErr: ErrMalformed,
		},
		// Missing job payload
		{
			resp: []byte(
				"+OK 1\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 1\r\n" +
					"\r\n",
			),
			expErr: ErrMalformed,
		},
		// Missing job payload with size greater than payload + \r\n
		// Triggers incomplete response read.
		{
			resp: []byte(
				"+OK 1\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 j1 10\r\n" +
					"\r\n",
			),
			expErr: ErrMalformed,
		},
	}
	tests = append(tests, invalidCommonErrorTests()...)

	for _, tt := range tests {
		conn := &TestConn{
			rdr: bytes.NewBuffer(tt.resp),
			wrt: bytes.NewBuffer([]byte("")),
		}
		client := NewClient(conn)
		j, err := client.Lease([]string{"j1"}, 1000)
		if j != nil || err == nil || tt.expErr == nil || err.Error() != tt.expErr.Error() {
			t.Fatalf("Response mismatch, err=%q, expErr=%q", err, tt.expErr)
		}
	}
}

func TestLeaseBadConnError(t *testing.T) {
	conn := &TestBadWriteConn{}
	client := NewClient(conn)
	j, err := client.Lease([]string{"j1"}, 1000)
	if _, ok := err.(*NetError); !ok {
		t.Fatalf("Error mismatch, err=%+v", err)
	}

	if j != nil {
		t.Fatalf("Response mismatch, job=%+v", j)
	}
}

func TestComplete(t *testing.T) {
	conn := &TestConn{
		rdr: bytes.NewBuffer([]byte("+OK\r\n")),
		wrt: bytes.NewBuffer([]byte("")),
	}
	client := NewClient(conn)
	err := client.Complete("6ba7b810-9dad-11d1-80b4-00c04fd430c4", []byte("a"))
	if err != nil {
		t.Fatalf("Response mismatch, err=%s", err)
	}

	expWrite := []byte(
		"complete 6ba7b810-9dad-11d1-80b4-00c04fd430c4 1\r\na\r\n",
	)
	if !bytes.Equal(expWrite, conn.wrt.Bytes()) {
		t.Fatalf("Write mismatch, act=%s", conn.wrt.Bytes())
	}
}

func TestCompleteErrors(t *testing.T) {
	var tests []RespErrTestCase
	tests = append(tests, invalidCommonErrorTests()...)

	for _, tt := range tests {
		conn := &TestConn{
			rdr: bytes.NewBuffer(tt.resp),
			wrt: bytes.NewBuffer([]byte("")),
		}
		client := NewClient(conn)
		err := client.Complete("6ba7b810-9dad-11d1-80b4-00c04fd430c4", []byte("a"))
		if err == nil || tt.expErr == nil || err.Error() != tt.expErr.Error() {
			t.Fatalf("Response mismatch, err=%q, expErr=%q", err, tt.expErr)
		}
	}
}

func TestCompleteBadConnError(t *testing.T) {
	conn := &TestBadWriteConn{}
	client := NewClient(conn)
	err := client.Complete("6ba7b810-9dad-11d1-80b4-00c04fd430c4", []byte("a"))
	if _, ok := err.(*NetError); !ok {
		t.Fatalf("Error mismatch, err=%+v", err)
	}
}

func TestFail(t *testing.T) {
	conn := &TestConn{
		rdr: bytes.NewBuffer([]byte("+OK\r\n")),
		wrt: bytes.NewBuffer([]byte("")),
	}
	client := NewClient(conn)
	err := client.Fail("6ba7b810-9dad-11d1-80b4-00c04fd430c4", []byte("a"))
	if err != nil {
		t.Fatalf("Response mismatch, err=%s", err)
	}

	expWrite := []byte(
		"fail 6ba7b810-9dad-11d1-80b4-00c04fd430c4 1\r\na\r\n",
	)
	if !bytes.Equal(expWrite, conn.wrt.Bytes()) {
		t.Fatalf("Write mismatch, act=%s", conn.wrt.Bytes())
	}
}

func TestFailErrors(t *testing.T) {
	var tests []RespErrTestCase
	tests = append(tests, invalidCommonErrorTests()...)

	for _, tt := range tests {
		conn := &TestConn{
			rdr: bytes.NewBuffer(tt.resp),
			wrt: bytes.NewBuffer([]byte("")),
		}
		client := NewClient(conn)
		err := client.Fail("6ba7b810-9dad-11d1-80b4-00c04fd430c4", []byte("a"))
		if err == nil || tt.expErr == nil || err.Error() != tt.expErr.Error() {
			t.Fatalf("Response mismatch, err=%q, expErr=%q", err, tt.expErr)
		}
	}
}

func TestFailBadConnError(t *testing.T) {
	conn := &TestBadWriteConn{}
	client := NewClient(conn)
	err := client.Fail("6ba7b810-9dad-11d1-80b4-00c04fd430c4", []byte("a"))
	if _, ok := err.(*NetError); !ok {
		t.Fatalf("Error mismatch, err=%+v", err)
	}
}

func TestDelete(t *testing.T) {
	conn := &TestConn{
		rdr: bytes.NewBuffer([]byte("+OK\r\n")),
		wrt: bytes.NewBuffer([]byte("")),
	}
	client := NewClient(conn)
	err := client.Delete("6ba7b810-9dad-11d1-80b4-00c04fd430c4")
	if err != nil {
		t.Fatalf("Response mismatch, err=%s", err)
	}

	expWrite := []byte(
		"delete 6ba7b810-9dad-11d1-80b4-00c04fd430c4\r\n",
	)
	if !bytes.Equal(expWrite, conn.wrt.Bytes()) {
		t.Fatalf("Write mismatch, act=%s", conn.wrt.Bytes())
	}
}

func TestDeleteErrors(t *testing.T) {
	var tests []RespErrTestCase
	tests = append(tests, invalidCommonErrorTests()...)

	for _, tt := range tests {
		conn := &TestConn{
			rdr: bytes.NewBuffer(tt.resp),
			wrt: bytes.NewBuffer([]byte("")),
		}
		client := NewClient(conn)
		err := client.Delete("6ba7b810-9dad-11d1-80b4-00c04fd430c4")
		if err == nil || tt.expErr == nil || err.Error() != tt.expErr.Error() {
			t.Fatalf("Response mismatch, err=%q, expErr=%q", err, tt.expErr)
		}
	}
}

func TestDeleteBadConnError(t *testing.T) {
	conn := &TestBadWriteConn{}
	client := NewClient(conn)
	err := client.Delete("6ba7b810-9dad-11d1-80b4-00c04fd430c4")
	if _, ok := err.(*NetError); !ok {
		t.Fatalf("Error mismatch, err=%+v", err)
	}
}

func TestInspectJobs(t *testing.T) {
	conn := &TestConn{
		rdr: bytes.NewBuffer([]byte(
			"+OK 2\r\n" +
				"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
				"name ping\r\n" +
				"ttr 1000\r\n" +
				"ttl 60000\r\n" +
				"payload-size 4\r\n" +
				"payload ping\r\n" +
				"max-attempts 0\r\n" +
				"attempts 0\r\n" +
				"max-fails 0\r\n" +
				"fails 0\r\n" +
				"priority 0\r\n" +
				"state 0\r\n" +
				"created 2016-08-22T01:50:51Z\r\n" +
				"6ba7b810-9dad-11d1-80b4-00c04fd430c6 12\r\n" +
				"name ping\r\n" +
				"ttr 1000\r\n" +
				"ttl 60000\r\n" +
				"payload-size 4\r\n" +
				"payload ping\r\n" +
				"max-attempts 0\r\n" +
				"attempts 0\r\n" +
				"max-fails 0\r\n" +
				"fails 0\r\n" +
				"priority 0\r\n" +
				"state 0\r\n" +
				"created 2016-08-22T02:00:17Z\r\n",
		)),
		wrt: bytes.NewBuffer([]byte("")),
	}
	client := NewClient(conn)
	jobs, err := client.InspectJobs("ping", 0, 10)
	if err != nil {
		t.Fatalf("Response mismatch, err=%s", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("Reply count mismatch")
	}

	if jobs[0].ID != "6ba7b810-9dad-11d1-80b4-00c04fd430c4" {
		t.Fatalf("ID mismatch")
	}
	if jobs[1].ID != "6ba7b810-9dad-11d1-80b4-00c04fd430c6" {
		t.Fatalf("ID mismatch")
	}
	if jobs[0].Name!= "ping" {
		t.Fatalf("Name mismatch")
	}
	if jobs[0].TTR!= 1000 {
		t.Fatalf("TTR mismatch")
	}
	if jobs[0].TTL!= 60000 {
		t.Fatalf("TTL mismatch")
	}
	if !bytes.Equal([]byte("ping"), jobs[0].Payload) {
		t.Fatalf("Payload mismatch")
	}
	if jobs[0].MaxAttempts!= 0 {
		t.Fatalf("MaxAttempts mismatch")
	}
	if jobs[0].Attempts!= 0 {
		t.Fatalf("Attempts mismatch")
	}
	if jobs[0].MaxFails!= 0 {
		t.Fatalf("MaxFails mismatch")
	}
	if jobs[0].Fails!= 0 {
		t.Fatalf("Fails mismatch")
	}
	if jobs[0].Priority!= 0 {
		t.Fatalf("Fails mismatch")
	}
	if jobs[0].State!= 0 {
		t.Fatalf("Fails mismatch")
	}
	timeRef := time.Date(2016, time.August, 22, 1, 50, 51, 0, time.UTC)
	if !time.Time.Equal(jobs[0].Created, timeRef) { // 2016-08-22T02:00:17Z
		t.Fatalf("Creation date mismatch: %s != %s", jobs[0].Created, timeRef)
	}

	expWrite := []byte(
		"inspect jobs ping 0 10\r\n",
	)
	if !bytes.Equal(expWrite, conn.wrt.Bytes()) {
		t.Fatalf("Write mismatch, act=%s", conn.wrt.Bytes())
	}
}

func TestInspectJobsErrors(t *testing.T) {
	tests := []RespErrTestCase{
		// Invalid reply-count
		{
			resp: []byte(
				"+OK 2\r\n" +
				"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n"+
				"name ping\r\n"+
				"ttr 1000\r\n"+
				"ttl 60000\r\n"+
				"payload-size 4\r\n"+
				"payload ping\r\n"+
				"max-attempts 0\r\n"+
				"attempts 0\r\n"+
				"max-fails 0\r\n"+
				"fails 0\r\n"+
				"priority 0\r\n"+
				"state 0\r\n"+
				"created 2016-08-22T01:50:51Z\r\n",
			),
			expErr: ErrMalformed,
		},
		// payload after payload size but with other keys in between
		{
			resp: []byte(
				"+OK 1\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n"+
					"name ping\r\n"+
					"ttr 1000\r\n"+
					"ttl 60000\r\n"+
					"payload-size 4\r\n"+
					"max-attempts 0\r\n"+
					"payload ping\r\n"+
					"attempts 0\r\n"+
					"max-fails 0\r\n"+
					"fails 0\r\n"+
					"priority 0\r\n"+
					"state 0\r\n"+
					"created 2016-08-22T01:50:51Z\r\n",
			),
			expErr: ErrPayloadMustFollowSize,
		},
		// payload before payload-size
		{
			resp: []byte(
				"+OK 1\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n"+
					"name ping\r\n"+
					"ttr 1000\r\n"+
					"payload ping\r\n"+
					"ttl 60000\r\n"+
					"payload-size 4\r\n"+
					"max-attempts 0\r\n"+
					"attempts 0\r\n"+
					"max-fails 0\r\n"+
					"fails 0\r\n"+
					"priority 0\r\n"+
					"state 0\r\n"+
					"created 2016-08-22T01:50:51Z\r\n",
			),
			expErr: ErrPayloadMustFollowSize,
		},
		// Invalid key-count (too small)
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 11\r\n" +
					"name ping\r\n" +
					"ttr 1000\r\n" +
					"ttl 60000\r\n" +
					"payload-size 4\r\n" +
					"payload ping\r\n" +
					"max-attempts 0\r\n" +
					"attempts 0\r\n" +
					"max-fails 0\r\n" +
					"fails 0\r\n" +
					"priority 0\r\n" +
					"state 0\r\n" +
					"created 2016-08-22T01:50:51Z\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c6 12\r\n" +
					"name ping\r\n" +
					"ttr 1000\r\n" +
					"ttl 60000\r\n" +
					"payload-size 4\r\n" +
					"payload ping\r\n" +
					"max-attempts 0\r\n" +
					"attempts 0\r\n" +
					"max-fails 0\r\n" +
					"fails 0\r\n" +
					"priority 0\r\n" +
					"state 0\r\n" +
					"created 2016-08-22T02:00:17Z\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid key-count (too small) on last reply
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"name ping\r\n" +
					"ttr 1000\r\n" +
					"ttl 60000\r\n" +
					"payload-size 4\r\n" +
					"payload ping\r\n" +
					"max-attempts 0\r\n" +
					"attempts 0\r\n" +
					"max-fails 0\r\n" +
					"fails 0\r\n" +
					"priority 0\r\n" +
					"state 0\r\n" +
					"created 2016-08-22T01:50:51Z\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c6 11\r\n" +
					"name ping\r\n" +
					"ttr 1000\r\n" +
					"ttl 60000\r\n" +
					"payload-size 4\r\n" +
					"payload ping\r\n" +
					"max-attempts 0\r\n" +
					"attempts 0\r\n" +
					"max-fails 0\r\n" +
					"fails 0\r\n" +
					"priority 0\r\n" +
					"state 0\r\n" +
					"created 2016-08-22T02:00:17Z\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid key-count (too large)
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 13\r\n" +
					"name ping\r\n" +
					"ttr 1000\r\n" +
					"ttl 60000\r\n" +
					"payload-size 4\r\n" +
					"payload ping\r\n" +
					"max-attempts 0\r\n" +
					"attempts 0\r\n" +
					"max-fails 0\r\n" +
					"fails 0\r\n" +
					"priority 0\r\n" +
					"state 0\r\n" +
					"created 2016-08-22T01:50:51Z\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c6 12\r\n" +
					"name ping\r\n" +
					"ttr 1000\r\n" +
					"ttl 60000\r\n" +
					"payload-size 4\r\n" +
					"payload ping\r\n" +
					"max-attempts 0\r\n" +
					"attempts 0\r\n" +
					"max-fails 0\r\n" +
					"fails 0\r\n" +
					"priority 0\r\n" +
					"state 0\r\n" +
					"created 2016-08-22T02:00:17Z\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid key-count (too large) on last reply
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"name ping\r\n" +
					"ttr 1000\r\n" +
					"ttl 60000\r\n" +
					"payload-size 4\r\n" +
					"payload ping\r\n" +
					"max-attempts 0\r\n" +
					"attempts 0\r\n" +
					"max-fails 0\r\n" +
					"fails 0\r\n" +
					"priority 0\r\n" +
					"state 0\r\n" +
					"created 2016-08-22T01:50:51Z\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c6 13\r\n" +
					"name ping\r\n" +
					"ttr 1000\r\n" +
					"ttl 60000\r\n" +
					"payload-size 4\r\n" +
					"payload ping\r\n" +
					"max-attempts 0\r\n" +
					"attempts 0\r\n" +
					"max-fails 0\r\n" +
					"fails 0\r\n" +
					"priority 0\r\n" +
					"state 0\r\n" +
					"created 2016-08-22T02:00:17Z\r\n",
			),
			expErr: ErrMalformed,
		},
		// Malformed "<id> <key-count>" line (no spaces)
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4\r\n",
			),
			expErr: ErrMalformed,
		},
		// Malformed "<id> <key-count>" line (too many spaces)
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12 abc\r\n",
			),
			expErr: ErrMalformed,
		},
		// Malformed "<id> <key-count>" line (key count not a number)
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 xy\r\n",
			),
			expErr: ErrMalformed,
		},
		// Malformed "<key> <value>" line (no spaces)
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"nameping\r\n",
			),
			expErr: ErrMalformed,
		},
		// Malformed "<key> <value>" line (spaces in value)
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"name pi ng\r\n",
			),
			expErr: ErrMalformed,
		},
		// Malformed "name <value>" line (illegal characters in value)
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"name pi*ng\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid TTR
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"ttr -1\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid TTL
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"ttl -1\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid payload-size
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"payload-size -1\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid payload
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"payload-size 10\r\n" +
					"payload abc\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid max-attempts
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"max-attempts -1\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid attempts
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"attempts -1\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid max-fails
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"max-fails -1\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid fails
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"fails -1\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid priority
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"priority xy\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid state
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"state xy\r\n",
			),
			expErr: ErrMalformed,
		},
		// Invalid created
		{
			resp: []byte(
				"+OK 2\r\n" +
					"6ba7b810-9dad-11d1-80b4-00c04fd430c4 12\r\n" +
					"created 20invalid16-08-22T02:00:17Z\r\n",
			),
			expErr: ErrMalformed,
		},
	}
	tests = append(tests, invalidCommonErrorTests()...)

	for _, tt := range tests {
		conn := &TestConn{
			rdr: bytes.NewBuffer(tt.resp),
			wrt: bytes.NewBuffer([]byte("")),
		}
		client := NewClient(conn)
		j, err := client.InspectJobs("ping", 0,10)
		if j != nil || err == nil || tt.expErr == nil || err.Error() != tt.expErr.Error() {
			t.Fatalf("Response mismatch, err=%q, expErr=%q", err, tt.expErr)
		}
	}
}

func TestInspectJobsBadConnError(t *testing.T) {
	conn := &TestBadWriteConn{}
	client := NewClient(conn)
	_, err := client.InspectJobs("ping", 0, 10)
	if _, ok := err.(*NetError); !ok {
		t.Fatalf("Error mismatch, err=%+v", err)
	}
}

type RespErrTestCase struct {
	resp   []byte
	expErr error
}

func invalidCommonErrorTests() []RespErrTestCase {
	return []RespErrTestCase{
		{
			resp:   []byte(""),
			expErr: NewNetError("EOF"),
		},
		{
			resp:   []byte("*OK\r\n"),
			expErr: ErrMalformed,
		},
		{
			resp:   []byte("-NOT-FOUND"),
			expErr: NewNetError("EOF"),
		},
		// Whitespace as code and text
		{
			resp:   []byte("-  \r\n"),
			expErr: ErrMalformed,
		},
		// Whitespace as code
		{
			resp:   []byte("- \r\n"),
			expErr: ErrMalformed,
		},
		// Whitespace as error text.
		{
			resp:   []byte("-C \r\n"),
			expErr: ErrMalformed,
		},
		{
			resp:   []byte("\n"),
			expErr: ErrMalformed,
		},
		{
			resp:   []byte("a\n"),
			expErr: ErrMalformed,
		},
		{
			resp:   []byte("\r\n"),
			expErr: ErrMalformed,
		},
		{
			resp:   []byte("NOT-FOUND\r\n"),
			expErr: ErrMalformed,
		},
		{
			resp:   []byte("NOT-FOUND"),
			expErr: NewNetError("EOF"),
		},
		{
			resp:   []byte("-NOT-FOUND\r\n"),
			expErr: NewResponseError("NOT-FOUND", ""),
		},
		{
			resp:   []byte("-TIMED-OUT\r\n"),
			expErr: NewResponseError("TIMED-OUT", ""),
		},
	}
}

func invalidResultErrorTests() []RespErrTestCase {
	return []RespErrTestCase{
		// Invalid reply-count
		{
			resp:   []byte("+OK 2\r\n6ba7b810-9dad-11d1-80b4-00c04fd430c4 0 1\r\na\r\n"),
			expErr: ErrMalformed,
		},
		// Missing result data
		{
			resp:   []byte("+OK 1\r\n6ba7b810-9dad-11d1-80b4-00c04fd430c4 0 1\r\n\r\n"),
			expErr: ErrMalformed,
		},
		// Missing result data with result-size greater than expected result + \r\n
		// Triggers incomplete read.
		{
			resp:   []byte("+OK 1\r\n6ba7b810-9dad-11d1-80b4-00c04fd430c4 0 10\r\n\r\n"),
			expErr: ErrMalformed,
		},
		{
			resp: []byte("+OK 2\r\n" +
				"6ba7b810-9dad-11d1-80b4-00c04fd430c4 1 1\r\n" +
				"a\r\n"),
			expErr: ErrMalformed,
		},
		{
			resp: []byte("+OK 1\r\n" +
				"6ba7b810-9dad-11d1-80b4-00c04fd430c4 1 1 1\r\n" +
				"a\r\n"),
			expErr: ErrMalformed,
		},
		{
			resp: []byte("+OK 1\r\n" +
				"6ba7b810-9dad-11d1-80b4-00c04fd430c4 1 -1\r\n" +
				"a\r\n"),
			expErr: ErrMalformed,
		},
		{
			resp: []byte("+OK 1\r\n" +
				"6ba7b810-9dad-11d1-80b4-00c04fd430c4 1 1048577\r\n" +
				"a\r\n"),
			expErr: ErrMalformed,
		},
		{
			resp: []byte("+OK 1\r\n" +
				"6ba7b810-9dad-11d1-80b4-00c04fd430c4 -1 1\r\n" +
				"a\r\n"),
			expErr: ErrMalformed,
		},
	}
}

type TestConn struct {
	rdr       *bytes.Buffer
	wrt       *bytes.Buffer
	proxyConn net.Conn
}

func (c *TestConn) Read(b []byte) (int, error) {
	return c.rdr.Read(b)
}

func (c *TestConn) Write(b []byte) (int, error) {
	return c.wrt.Write(b)
}

func (c *TestConn) Close() error {
	return nil
}

func (c *TestConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *TestConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *TestConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *TestConn) LocalAddr() net.Addr {
	return &TestAddr{}
}

func (c *TestConn) RemoteAddr() net.Addr {
	return &TestAddr{}
}

type TestAddr struct{}

func (a *TestAddr) Network() string {
	return ""
}

func (a *TestAddr) String() string {
	return ""
}

type TestBadWriteConn struct {
}

func (c *TestBadWriteConn) Read(b []byte) (int, error) {
	return 0, nil
}

func (c *TestBadWriteConn) Write(b []byte) (int, error) {
	return 0, errors.New("A bad time")
}

func (c *TestBadWriteConn) Close() error {
	return nil
}

func (c *TestBadWriteConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *TestBadWriteConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *TestBadWriteConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *TestBadWriteConn) LocalAddr() net.Addr {
	return &TestAddr{}
}

func (c *TestBadWriteConn) RemoteAddr() net.Addr {
	return &TestAddr{}
}
