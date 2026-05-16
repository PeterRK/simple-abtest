package main

import "testing"

func TestValidResultWriteRequestMultiMetric(t *testing.T) {
	req := &resultWriteRequest{
		LayerName:  "feed_rank",
		BucketType: bucketTypeHour,
		MetricName: []string{"ctr", "conversion_rate"},
		Points: []resultWritePoint{
			{
				GroupName:    "control",
				BucketKey:    "2026051413",
				BucketStamp:  1778754000,
				MetricValues: []float64{0.123, 0.041},
			},
			{
				GroupName:    "variant_b",
				BucketKey:    "2026051413",
				BucketStamp:  1778754000,
				MetricValues: []float64{0.137, 0.046},
			},
		},
	}
	if !validResultWriteRequest(req) {
		t.Fatal("expected valid multi-metric result write request")
	}
}

func TestValidResultWriteRequestRejectsInvalidMetricShape(t *testing.T) {
	base := &resultWriteRequest{
		LayerName:  "feed_rank",
		BucketType: bucketTypeHour,
		MetricName: []string{"ctr", "conversion_rate"},
		Points: []resultWritePoint{
			{
				GroupName:    "control",
				BucketKey:    "2026051413",
				BucketStamp:  1778754000,
				MetricValues: []float64{0.123, 0.041},
			},
		},
	}

	cases := []struct {
		name string
		req  *resultWriteRequest
	}{
		{
			name: "duplicate metric",
			req: &resultWriteRequest{
				LayerName:  base.LayerName,
				BucketType: base.BucketType,
				MetricName: []string{"ctr", "ctr"},
				Points:     base.Points,
			},
		},
		{
			name: "mismatched values",
			req: &resultWriteRequest{
				LayerName:  base.LayerName,
				BucketType: base.BucketType,
				MetricName: base.MetricName,
				Points: []resultWritePoint{
					{
						GroupName:    "control",
						BucketKey:    "2026051413",
						BucketStamp:  1778754000,
						MetricValues: []float64{0.123},
					},
				},
			},
		},
		{
			name: "expanded row limit",
			req: &resultWriteRequest{
				LayerName:  base.LayerName,
				BucketType: base.BucketType,
				MetricName: base.MetricName,
				Points:     make([]resultWritePoint, maxResultWriteRowsPerRequest/len(base.MetricName)+1),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if validResultWriteRequest(tc.req) {
				t.Fatal("expected invalid result write request")
			}
		})
	}
}

func TestResultWriteBatchStmtSelectsLargestFit(t *testing.T) {
	old := resultSql.upsert
	resultSql.upsert = []resultUpsertStmt{
		{size: 1},
		{size: 2},
		{size: 4},
		{size: 8},
		{size: 16},
		{size: 32},
		{size: 64},
		{size: 128},
		{size: 256},
	}
	defer func() { resultSql.upsert = old }()

	cases := []struct {
		remaining int
		want      int
	}{
		{remaining: 0, want: 0},
		{remaining: 1, want: 1},
		{remaining: 3, want: 2},
		{remaining: 63, want: 32},
		{remaining: 180, want: 128},
		{remaining: 256, want: 256},
		{remaining: 10000, want: 256},
	}

	for _, tc := range cases {
		got, _ := resultWriteBatchStmt(tc.remaining)
		if got != tc.want {
			t.Fatalf("resultWriteBatchStmt(%d) size=%d, want %d", tc.remaining, got, tc.want)
		}
	}
}
