package license

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/ruko1202/xlog"

	"github.com/ruko1202/maintmode/internal/entity"
)

// Send posts the heartbeat and returns the license Console granted.
// FetchedAt is left zero — the caller stamps it. Any transport error, non-200
// status or malformed/invalid body is returned as an error; the caller treats
// every error the same way: warn and keep the cached license.
func (c *Client) Send(ctx context.Context, report *entity.HeartbeatReport) (*entity.License, error) {
	ctx, span := xlog.WithOperationSpan(ctx, "license.Send")
	defer span.End()

	req, err := toHeartbeatRequest(ctx, c.baseURL, report)
	if err != nil {
		return nil, fmt.Errorf("build heartbeat request: %w", err)
	}

	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send heartbeat: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("heartbeat returned status %d", resp.StatusCode)
	}

	var decoded heartbeatResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode heartbeat response: %w", err)
	}

	// A heartbeat response always carries seats_purchased, so it is non-nil from
	// here on; nil SeatsPurchased only ever means "no heartbeat has landed yet".
	return &entity.License{
		Status:         entity.LicenseStatus(decoded.Status),
		SeatsPurchased: &decoded.SeatsPurchased,
	}, nil
}

func toHeartbeatSeatBucket(b entity.SeatBucket) heartbeatSeatBucket {
	return heartbeatSeatBucket{
		Active:  b.Active,
		Pending: b.Pending,
	}
}

func toHeartbeatRequest(ctx context.Context, baseURL string, report *entity.HeartbeatReport) (*http.Request, error) {
	body, err := json.Marshal(heartbeatRequest{
		PerRole: heartbeatPerRole{
			Admin:    toHeartbeatSeatBucket(report.SeatsUsed.Admin),
			Reviewer: toHeartbeatSeatBucket(report.SeatsUsed.Reviewer),
			Editor:   toHeartbeatSeatBucket(report.SeatsUsed.Editor),
			Guest:    toHeartbeatSeatBucket(report.SeatsUsed.Guest),
		},
		SeatsUsed:      report.SeatsUsed.SeatsUsed(),
		LastActivityAt: report.LastActivityAt,
		Version:        report.Version,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal heartbeat request: %w", err)
	}

	u, err := url.JoinPath(baseURL, heartbeatPath)
	if err != nil {
		return nil, fmt.Errorf("build heartbeat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build heartbeat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}
