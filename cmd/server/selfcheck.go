package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"stage-rigging-release/internal/domain"
	"time"
)

type checkBatchResponse struct {
	Batch domain.InspectionBatch `json:"batch"`
}

func runSelfcheck(ctx context.Context, base string) error {
	client := &http.Client{Timeout: 4 * time.Second}
	var created checkBatchResponse
	performance := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	if err := checkJSON(ctx, client, "POST", base+"/api/v1/batches", map[string]any{"expectedVersion": 0, "idempotencyKey": "check-create", "venueName": "自检剧场", "stageZone": "主舞台", "performanceAt": performance, "ownerName": "自检负责人"}, &created); err != nil {
		return fmt.Errorf("创建批次: %w", err)
	}
	b := &created.Batch
	var pointResult checkBatchResponse
	if err := checkJSON(ctx, client, "POST", base+"/api/v1/batches/"+b.ID+"/points", map[string]any{"expectedVersion": b.Version, "idempotencyKey": "check-point", "pointCode": "P-01", "hoistSerial": "H-0001", "ropeSpec": "6x19-12mm", "ratedLoadKg": 1000, "plannedLoadKg": 600, "positionNote": "台口中线"}, &pointResult); err != nil {
		return fmt.Errorf("登记吊点: %w", err)
	}
	b = &pointResult.Batch
	pointID := b.Points[0].ID
	if err := checkMutation(ctx, client, base, b, "lock", map[string]any{"expectedVersion": b.Version, "idempotencyKey": "check-lock"}); err != nil {
		return err
	}
	b = lastCheckBatch
	failed := map[string]any{"expectedVersion": b.Version, "idempotencyKey": "check-failed-test", "riggingPointId": pointID, "targetLoadKg": 750, "measuredLoadKg": 700, "holdSeconds": 45, "displacementMm": 6.2, "recordedBy": "记录员甲", "deviation": map[string]any{"severity": "major", "symptom": "保持阶段位移超限", "requiredAction": "复核绳夹并重新张紧", "assignee": "机械技师甲", "assigneeConfirmed": true, "confirmedBy": "机械技师甲"}}
	if err := checkMutation(ctx, client, base, b, "tests", failed); err != nil {
		return err
	}
	b = lastCheckBatch
	if b.Status != domain.StatusBlocked || len(b.Deviations) != 1 {
		return fmt.Errorf("失败试验未形成阻断偏差")
	}
	deviationID := b.Deviations[0].ID
	if err := checkMutation(ctx, client, base, b, "deviations/"+deviationID+"/remediation", map[string]any{"expectedVersion": b.Version, "idempotencyKey": "check-remediation", "evidence": "绳夹按扭矩复紧，附现场复核记录"}); err != nil {
		return err
	}
	b = lastCheckBatch
	if err := checkMutation(ctx, client, base, b, "deviations/"+deviationID+"/retest", map[string]any{"expectedVersion": b.Version, "idempotencyKey": "check-retest", "targetLoadKg": 750, "measuredLoadKg": 755, "holdSeconds": 90, "displacementMm": 1.1, "recordedBy": "记录员甲"}); err != nil {
		return err
	}
	b = lastCheckBatch
	if b.Status != domain.StatusPendingReview {
		return fmt.Errorf("复测闭环后状态为 %s", b.Status)
	}
	if err := checkMutation(ctx, client, base, b, "approval", map[string]any{"expectedVersion": b.Version, "idempotencyKey": "check-approval", "approvedBy": "技术负责人乙", "approvalNote": "清单完整，试验覆盖和偏差闭环符合放行要求"}); err != nil {
		return err
	}
	b = lastCheckBatch
	if err := checkMutation(ctx, client, base, b, "credential", map[string]any{"expectedVersion": b.Version, "idempotencyKey": "check-issue", "approvedBy": "技术负责人乙", "approvalNote": "批准本场演出使用冻结吊挂配置"}); err != nil {
		return err
	}
	b = lastCheckBatch
	var verification struct {
		Status string `json:"status"`
	}
	if err := checkJSON(ctx, client, "GET", base+"/api/v1/batches/"+b.ID+"/verification", nil, &verification); err != nil {
		return fmt.Errorf("核验凭据: %w", err)
	}
	if verification.Status != "valid" {
		return fmt.Errorf("摘要核验状态为 %s", verification.Status)
	}
	var replay checkBatchResponse
	if err := checkJSON(ctx, client, "POST", base+"/api/v1/batches", map[string]any{"expectedVersion": 0, "idempotencyKey": "check-create", "venueName": "重试内容不应生效", "stageZone": "主舞台", "performanceAt": performance, "ownerName": "自检负责人"}, &replay); err != nil {
		return fmt.Errorf("幂等重放: %w", err)
	}
	if replay.Batch.ID != created.Batch.ID {
		return fmt.Errorf("幂等重放返回了不同批次")
	}
	return nil
}

var lastCheckBatch *domain.InspectionBatch

func checkMutation(ctx context.Context, client *http.Client, base string, b *domain.InspectionBatch, suffix string, body map[string]any) error {
	var response checkBatchResponse
	if err := checkJSON(ctx, client, "POST", base+"/api/v1/batches/"+b.ID+"/"+suffix, body, &response); err != nil {
		return fmt.Errorf("调用 %s: %w", suffix, err)
	}
	lastCheckBatch = &response.Batch
	return nil
}

func checkJSON(ctx context.Context, client *http.Client, method, url string, body any, dst any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw))
	}
	if dst != nil && len(raw) > 0 {
		return json.Unmarshal(raw, dst)
	}
	return nil
}
