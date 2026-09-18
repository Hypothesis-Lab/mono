package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"app/internal/clients/yandex_direct"

	_ "modernc.org/sqlite"
)

const (
	outputDirectory = "/tmp/yandex_direct_inspect"
	outputFile      = "yandex_direct.sqlite"
)

type objectGroup struct {
	objectType string
	objects    []json.RawMessage
}

func main() {
	token := os.Getenv("YANDEX_DIRECT_TOKEN")
	if token == "" {
		log.Fatal("YANDEX_DIRECT_TOKEN is required")
	}

	ctx := context.Background()
	client := yandexdirect.New(token)

	campaigns, err := listAll(ctx, client, "campaigns", "Campaigns", map[string]any{"Types": []string{"CPM_BANNER_CAMPAIGN"}}, []string{"Id", "Name", "StartDate", "EndDate", "Type", "Status", "State", "StatusPayment", "StatusClarification"}, map[string][]string{"CpmBannerCampaignFieldNames": {"BiddingStrategy"}})
	if err != nil {
		log.Fatalf("list campaigns: %v", err)
	}

	adGroups, err := listByIDs(ctx, client, "adgroups", "AdGroups", "CampaignIds", objectIDs(campaigns), 10, []string{"Id", "Name", "CampaignId", "RegionIds", "Status", "ServingStatus", "Type", "Subtype"})
	if err != nil {
		log.Fatalf("list ad groups: %v", err)
	}

	retargetingLists, err := listAll(ctx, client, "retargetinglists", "RetargetingLists", map[string]any{"Types": []string{"AUDIENCE"}}, []string{"Id", "Name", "Type", "Description", "Rules", "IsAvailable", "Scope", "AvailableForTargetsInAdGroupTypes"}, nil)
	if err != nil {
		log.Fatalf("list retargeting lists: %v", err)
	}

	audienceTargets, err := listByIDs(ctx, client, "audiencetargets", "AudienceTargets", "AdGroupIds", objectIDs(adGroups), 1000, []string{"Id", "AdGroupId", "CampaignId", "RetargetingListId", "InterestId", "State", "ContextBid", "StrategyPriority"})
	if err != nil {
		log.Fatalf("list audience targets by ad group: %v", err)
	}

	targetsByList, err := listByIDs(ctx, client, "audiencetargets", "AudienceTargets", "RetargetingListIds", objectIDs(retargetingLists), 1000, []string{"Id", "AdGroupId", "CampaignId", "RetargetingListId", "InterestId", "State", "ContextBid", "StrategyPriority"})
	if err != nil {
		log.Fatalf("list audience targets by retargeting list: %v", err)
	}

	audienceTargets = uniqueObjects(append(audienceTargets, targetsByList...))

	groups := []objectGroup{
		{objectType: "campaign", objects: campaigns},
		{objectType: "ad_group", objects: adGroups},
		{objectType: "retargeting_list", objects: retargetingLists},
		{objectType: "audience_target", objects: audienceTargets},
	}

	path, err := writeDatabase(groups)
	if err != nil {
		log.Fatalf("write inspection database: %v", err)
	}

	for _, group := range groups {
		fmt.Printf("%s: %d\n", group.objectType, len(group.objects))
	}

	fmt.Printf("saved inspection to %s\n", path)
}

func listAll(ctx context.Context, client *yandexdirect.Client, service, objectName string, selectionCriteria map[string]any, fieldNames []string, additionalFields map[string][]string) ([]json.RawMessage, error) {
	return yandexdirect.ListAll(ctx, client, service, objectName, yandexdirect.GetOptions{
		SelectionCriteria: selectionCriteria,
		FieldNames:        fieldNames,
		AdditionalFields:  additionalFields,
	})
}

func listByIDs(ctx context.Context, client *yandexdirect.Client, service, objectName, criterion string, ids []int64, batchSize int, fieldNames []string) ([]json.RawMessage, error) {
	var objects []json.RawMessage
	for start := 0; start < len(ids); start += batchSize {
		end := min(start+batchSize, len(ids))
		items, err := listAll(ctx, client, service, objectName, map[string]any{criterion: ids[start:end]}, fieldNames, nil)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", objectName, err)
		}

		objects = append(objects, items...)
	}

	return objects, nil
}

func objectIDs(objects []json.RawMessage) []int64 {
	ids := make([]int64, 0, len(objects))
	for _, object := range objects {
		var item struct {
			ID int64 `json:"Id"`
		}
		if err := json.Unmarshal(object, &item); err != nil {
			log.Fatalf("decode object id: %v", err)
		}

		if item.ID == 0 {
			log.Fatal("object has no id")
		}

		ids = append(ids, item.ID)
	}

	return ids
}

func uniqueObjects(objects []json.RawMessage) []json.RawMessage {
	unique := make(map[int64]struct{}, len(objects))
	items := make([]json.RawMessage, 0, len(objects))
	for _, object := range objects {
		var item struct {
			ID int64 `json:"Id"`
		}
		if err := json.Unmarshal(object, &item); err != nil {
			log.Fatalf("decode object id: %v", err)
		}

		if item.ID == 0 {
			log.Fatal("object has no id")
		}

		if _, ok := unique[item.ID]; !ok {
			unique[item.ID] = struct{}{}
			items = append(items, object)
		}
	}

	return items
}

func writeDatabase(groups []objectGroup) (string, error) {
	if err := os.MkdirAll(outputDirectory, 0o750); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	path := filepath.Join(outputDirectory, outputFile)
	temporaryPath := path + ".tmp"
	if err := os.Remove(temporaryPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("remove previous temporary database: %w", err)
	}

	defer func() { _ = os.Remove(temporaryPath) }()

	if err := writeObjects(temporaryPath, groups); err != nil {
		return "", err
	}

	if err := os.Rename(temporaryPath, path); err != nil {
		return "", fmt.Errorf("replace inspection database: %w", err)
	}

	return path, nil
}

func writeObjects(path string, groups []objectGroup) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open temporary database: %w", err)
	}

	defer func() { _ = db.Close() }()

	if _, err := db.Exec(`
		CREATE TABLE objects (
			id INTEGER PRIMARY KEY,
			object_type TEXT NOT NULL,
			object_id INTEGER NOT NULL,
			raw_object TEXT NOT NULL,
			UNIQUE (object_type, object_id)
		)
	`); err != nil {
		return fmt.Errorf("create objects table: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin objects transaction: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	statement, err := tx.Prepare(`INSERT INTO objects (object_type, object_id, raw_object) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare objects insert: %w", err)
	}

	defer func() { _ = statement.Close() }()

	for _, group := range groups {
		for _, object := range group.objects {
			var item struct {
				ID int64 `json:"Id"`
			}
			if err := json.Unmarshal(object, &item); err != nil {
				return fmt.Errorf("decode %s id: %w", group.objectType, err)
			}

			if item.ID == 0 {
				return fmt.Errorf("%s has no id", group.objectType)
			}

			if _, err := statement.Exec(group.objectType, item.ID, string(object)); err != nil {
				return fmt.Errorf("insert %s %d: %w", group.objectType, item.ID, err)
			}
		}
	}

	if err := statement.Close(); err != nil {
		return fmt.Errorf("close objects insert: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit objects transaction: %w", err)
	}

	if err := db.Close(); err != nil {
		return fmt.Errorf("close temporary database: %w", err)
	}

	return nil
}
