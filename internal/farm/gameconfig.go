package farm

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const levelShopJumpTarget = "[11]"

var gameConfigRootOverride struct {
	sync.RWMutex
	root string
}

func SetGameConfigRoot(root string) {
	gameConfigRootOverride.Lock()
	defer gameConfigRootOverride.Unlock()
	gameConfigRootOverride.root = strings.TrimSpace(root)
}

var growPhaseDurationPattern = regexp.MustCompile(`:(\d+)$`)

var warehouseCategoryOrder = []WarehouseCategorySummary{
	{Key: "fruit", Label: "果实"},
	{Key: "mutation", Label: "超变果实"},
	{Key: "seed", Label: "种子"},
	{Key: "tool", Label: "道具"},
}

type CropAnalyticsPayload struct {
	Source            string                    `json:"source"`
	Items             []CropAnalyticsItem       `json:"items"`
	Sort              string                    `json:"sort"`
	RequestedMaxLevel int                       `json:"requestedMaxLevel"`
	EffectiveMaxLevel int                       `json:"effectiveMaxLevel"`
	LevelSource       string                    `json:"levelSource"`
	Profile           map[string]any            `json:"profile,omitempty"`
	Strategies        []PlantStrategyMode       `json:"strategies"`
	Recommendations   []PlantRecommendationCard `json:"recommendations"`
	RuntimeError      string                    `json:"runtimeError,omitempty"`
	Error             string                    `json:"error,omitempty"`
}

type CropAnalyticsItem struct {
	ID                            int     `json:"id"`
	SeedID                        int     `json:"seedId"`
	Name                          string  `json:"name"`
	Seasons                       int     `json:"seasons"`
	Level                         *int    `json:"level,omitempty"`
	GrowTime                      int     `json:"growTime"`
	GrowTimeText                  string  `json:"growTimeText"`
	ReduceSecApplied              int     `json:"reduceSecApplied"`
	HarvestExp                    int     `json:"harvestExp"`
	ExpPerHour                    float64 `json:"expPerHour"`
	NormalFertilizerExpPerHour    float64 `json:"normalFertilizerExpPerHour"`
	Income                        int     `json:"income"`
	NetProfit                     int     `json:"netProfit"`
	ProfitPerHour                 float64 `json:"profitPerHour"`
	NormalFertilizerProfitPerHour float64 `json:"normalFertilizerProfitPerHour"`
	FruitID                       int     `json:"fruitId"`
	FruitCount                    int     `json:"fruitCount"`
	FruitPrice                    int     `json:"fruitPrice"`
	SeedPrice                     int     `json:"seedPrice"`
	PlantSize                     int     `json:"plantSize"`
	ShopEligible                  bool    `json:"shopEligible"`
	canonicalOrder                int
}

type BackpackSeedOptionsRequest struct {
	SelectedSeedIDs      []int `json:"selectedSeedIds"`
	DisabledSeedIDs      []int `json:"disabledSeedIds"`
	ForcePriority        bool  `json:"forcePriority"`
	FourGridPlantEnabled bool  `json:"fourGridPlantEnabled"`
}

type BackpackSeedOptionsPayload struct {
	OK        bool                 `json:"ok"`
	UpdatedAt int64                `json:"updatedAt"`
	List      []BackpackSeedOption `json:"list"`
	Error     string               `json:"error,omitempty"`
}

type StealCropOptionsPayload struct {
	OK        bool              `json:"ok"`
	UpdatedAt int64             `json:"updatedAt"`
	List      []StealCropOption `json:"list"`
	Error     string            `json:"error,omitempty"`
}

type BackpackSeedOption struct {
	SeedID                  int    `json:"seedId"`
	Name                    string `json:"name"`
	Level                   *int   `json:"level,omitempty"`
	Count                   int    `json:"count"`
	BackpackCount           int    `json:"backpackCount"`
	InBackpack              bool   `json:"inBackpack"`
	Disabled                bool   `json:"disabled"`
	Plantable               bool   `json:"plantable"`
	PlantableReason         string `json:"plantableReason,omitempty"`
	PlantableMessage        string `json:"plantableMessage,omitempty"`
	PlantSize               int    `json:"plantSize"`
	RetainedByForcePriority bool   `json:"retainedByForcePriority"`
}

type StealCropOption struct {
	PlantID   int    `json:"plantId"`
	SeedID    int    `json:"seedId,omitempty"`
	Name      string `json:"name"`
	Level     int    `json:"level,omitempty"`
	ImageURL  string `json:"imageUrl,omitempty"`
	SortGroup int    `json:"sortGroup,omitempty"`
	SortOrder int    `json:"sortOrder,omitempty"`
}

type PlantStrategyMode struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	NeedsSeedID bool   `json:"needsSeedId"`
}

type PlantRecommendationCard struct {
	Value                  string             `json:"value"`
	Label                  string             `json:"label"`
	Recommended            *CropAnalyticsItem `json:"recommended,omitempty"`
	CurrentRecommended     *CropAnalyticsItem `json:"currentRecommended,omitempty"`
	CurrentSource          string             `json:"currentSource,omitempty"`
	TheoreticalRecommended *CropAnalyticsItem `json:"theoreticalRecommended,omitempty"`
}

type AtlasPreviewPayload struct {
	Source         string         `json:"source"`
	Status         string         `json:"status"`
	Message        string         `json:"message"`
	RefreshEnabled bool           `json:"refreshEnabled"`
	BuyEnabled     bool           `json:"buyEnabled"`
	Sections       []AtlasSection `json:"sections"`
	Error          string         `json:"error,omitempty"`
}

type AtlasSection struct {
	ID      string              `json:"id"`
	Label   string              `json:"label"`
	Items   []AtlasItem         `json:"items"`
	Total   int                 `json:"total"`
	Summary AtlasSectionSummary `json:"summary"`
}

type AtlasItem struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	SeedID      int    `json:"seedId"`
	FruitID     int    `json:"fruitId"`
	GroupName   string `json:"groupName,omitempty"`
	FruitType   int    `json:"fruitType,omitempty"`
	FruitLayer  int    `json:"fruitLayer,omitempty"`
	FruitRarity int    `json:"fruitRarity,omitempty"`
	Progress    int    `json:"progress,omitempty"`
	Level       int    `json:"level"`
	Seasons     int    `json:"seasons"`
	GrowTime    int    `json:"growTime"`
	Locked      bool   `json:"locked"`
	Unlocked    bool   `json:"unlocked"`
	CanUpgrade  *bool  `json:"canUpgrade,omitempty"`
	IsNew       *bool  `json:"isNew,omitempty"`
	Sort        int    `json:"sort,omitempty"`
	AtlasType   string `json:"atlasType,omitempty"`
	ImageURL    string `json:"imageUrl,omitempty"`
}

type AtlasSectionSummary struct {
	Total    int `json:"total"`
	Unlocked int `json:"unlocked"`
	Locked   int `json:"locked"`
}

type AtlasLockedCropPurchaseInput struct {
	AtlasItems     []AtlasItem
	ShopList       []any
	EffectiveLevel int
	CountPerSeed   int
}

type AtlasPurchasePlan struct {
	Purchases []AtlasSeedPurchase      `json:"purchases"`
	Skipped   []AtlasPurchaseSkipped   `json:"skipped"`
	Summary   AtlasPurchasePlanSummary `json:"summary"`
}

type AtlasSeedPurchase struct {
	SeedID        int    `json:"seedId"`
	SeedName      string `json:"seedName"`
	GoodsID       int    `json:"goodsId"`
	Price         int    `json:"price"`
	Count         int    `json:"count"`
	RequiredLevel int    `json:"requiredLevel"`
	Sort          int    `json:"sort"`
}

type AtlasPurchaseSkipped struct {
	Name           string `json:"name,omitempty"`
	SeedID         int    `json:"seedId,omitempty"`
	FruitID        int    `json:"fruitId,omitempty"`
	Sort           int    `json:"sort,omitempty"`
	Reason         string `json:"reason"`
	RequiredLevel  int    `json:"requiredLevel,omitempty"`
	EffectiveLevel int    `json:"effectiveLevel,omitempty"`
}

type AtlasPurchasePlanSummary struct {
	Requested       int            `json:"requested"`
	LockedCropCount int            `json:"lockedCropCount"`
	Purchasable     int            `json:"purchasable"`
	Skipped         int            `json:"skipped"`
	SkipReasons     map[string]int `json:"skipReasons"`
	CountPerSeed    int            `json:"countPerSeed"`
	EffectiveLevel  int            `json:"effectiveLevel"`
}

type AtlasPurchasePreviewPayload struct {
	OK      bool               `json:"ok"`
	Error   string             `json:"error,omitempty"`
	Profile map[string]any     `json:"profile,omitempty"`
	Level   AnalyticsLevelInfo `json:"level"`
	Plan    AtlasPurchasePlan  `json:"plan"`
}

type AtlasPurchasePayload struct {
	OK       bool               `json:"ok"`
	Error    string             `json:"error,omitempty"`
	Profile  map[string]any     `json:"profile,omitempty"`
	Level    AnalyticsLevelInfo `json:"level"`
	Plan     AtlasPurchasePlan  `json:"plan"`
	Purchase map[string]any     `json:"purchase,omitempty"`
}

type RuntimeActionGate struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason"`
}

type LandDetailsPayload struct {
	Status       string              `json:"status"`
	Message      string              `json:"message"`
	Revision     string              `json:"revision,omitempty"`
	FarmType     string              `json:"farmType,omitempty"`
	TotalGrids   int                 `json:"totalGrids,omitempty"`
	Lands        []LandDetailsItem   `json:"lands"`
	Actions      []RuntimeActionGate `json:"actions"`
	RuntimeError string              `json:"runtimeError,omitempty"`
}

type LandDetailsDeltaPayload struct {
	Full           bool                `json:"full"`
	Revision       string              `json:"revision"`
	Status         string              `json:"status,omitempty"`
	Message        string              `json:"message,omitempty"`
	FarmType       string              `json:"farmType,omitempty"`
	TotalGrids     int                 `json:"totalGrids,omitempty"`
	Lands          []LandDetailsItem   `json:"lands"`
	RemovedLandIDs []int               `json:"removedLandIds"`
	Actions        []RuntimeActionGate `json:"actions,omitempty"`
	RuntimeError   string              `json:"runtimeError,omitempty"`
}

type LandDetailsItem struct {
	ID                       string                 `json:"id"`
	LandID                   int                    `json:"landId"`
	LandLevel                *int                   `json:"landLevel,omitempty"`
	PlantID                  int                    `json:"plantId,omitempty"`
	SeedID                   int                    `json:"seedId,omitempty"`
	LandType                 string                 `json:"landType,omitempty"`
	LandTypeLabel            string                 `json:"landTypeLabel,omitempty"`
	PlantName                string                 `json:"plantName,omitempty"`
	DisplayPlantName         string                 `json:"displayPlantName,omitempty"`
	ImageURL                 string                 `json:"imageUrl,omitempty"`
	Status                   string                 `json:"status"`
	StatusLabel              string                 `json:"statusLabel"`
	MatureInSec              *int                   `json:"matureInSec,omitempty"`
	MatureAtMs               int64                  `json:"matureAtMs,omitempty"`
	MatureEtaText            string                 `json:"matureEtaText,omitempty"`
	CurrentSeason            int                    `json:"currentSeason,omitempty"`
	TotalSeason              int                    `json:"totalSeason,omitempty"`
	CurrentStage             int                    `json:"currentStage,omitempty"`
	PhaseName                string                 `json:"phaseName,omitempty"`
	IsMultiSeason            bool                   `json:"isMultiSeason,omitempty"`
	LandSize                 int                    `json:"landSize,omitempty"`
	OccupancyPlantSize       int                    `json:"occupancyPlantSize,omitempty"`
	OccupancyAnchorLandID    int                    `json:"occupancyAnchorLandId,omitempty"`
	OccupiedByMultiTilePlant bool                   `json:"occupiedByMultiTilePlant,omitempty"`
	HasMutation              bool                   `json:"hasMutation,omitempty"`
	MutationLabel            string                 `json:"mutationLabel,omitempty"`
	MutationIconURL          string                 `json:"mutationIconUrl,omitempty"`
	MutationImageURL         string                 `json:"mutationImageUrl,omitempty"`
	MutationTypes            []LandMutationTypeItem `json:"mutationTypes,omitempty"`
	NeedWater                bool                   `json:"needWater,omitempty"`
	NeedWeed                 bool                   `json:"needWeed,omitempty"`
	NeedBug                  bool                   `json:"needBug,omitempty"`
	NeedGoldenBug            bool                   `json:"needGoldenBug,omitempty"`
	NeedEraseDead            bool                   `json:"needEraseDead,omitempty"`
	CanHarvest               bool                   `json:"canHarvest"`
}

func LandStateSignature(land LandDetailsItem) string {
	land.MatureInSec = nil
	land.MatureAtMs = 0
	land.MatureEtaText = ""
	encoded, err := json.Marshal(land)
	if err != nil {
		return ""
	}
	return string(encoded)
}

type LandMutationTypeItem struct {
	TypeID  int    `json:"typeId,omitempty"`
	Name    string `json:"name"`
	IconURL string `json:"iconUrl,omitempty"`
}

type WarehousePayload struct {
	Status       string              `json:"status"`
	Message      string              `json:"message"`
	Items        []WarehouseItem     `json:"items"`
	Actions      []RuntimeActionGate `json:"actions"`
	Summary      WarehouseSummary    `json:"summary"`
	RuntimeError string              `json:"runtimeError,omitempty"`
}

type WarehouseSellPayload struct {
	OK        bool             `json:"ok"`
	Error     string           `json:"error,omitempty"`
	Warehouse WarehousePayload `json:"warehouse"`
	Sell      map[string]any   `json:"sell,omitempty"`
}

type WarehouseItem struct {
	ID                 string `json:"id"`
	ItemID             int    `json:"itemId"`
	Name               string `json:"name"`
	Count              int    `json:"count"`
	Category           string `json:"category"`
	CategoryLabel      string `json:"categoryLabel"`
	ImageURL           string `json:"imageUrl,omitempty"`
	CanSell            bool   `json:"canSell"`
	Locked             bool   `json:"locked"`
	EstimatedSellPrice int    `json:"estimatedSellPrice"`
}

type WarehouseSummary struct {
	TotalDistinct         int                        `json:"totalDistinct"`
	TotalCount            int                        `json:"totalCount"`
	SellableDistinct      int                        `json:"sellableDistinct"`
	SellableCount         int                        `json:"sellableCount"`
	EstimatedAllSellPrice int                        `json:"estimatedAllSellPrice"`
	CategoryList          []WarehouseCategorySummary `json:"categoryList,omitempty"`
}

type WarehouseCategorySummary struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Distinct int    `json:"distinct"`
	Count    int    `json:"count"`
}

type plantConfigItem struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Fruit             fruit  `json:"fruit"`
	SeedID            int    `json:"seed_id"`
	LandLevelNeed     int    `json:"land_level_need"`
	Seasons           int    `json:"seasons"`
	GrowPhases        string `json:"grow_phases"`
	Exp               int    `json:"exp"`
	Size              int    `json:"size"`
	MutantEffectPlant string `json:"mutant_effect_plant"`
}

type SmartFertilizerCropClass int

const (
	SmartFertilizerCropUnknown SmartFertilizerCropClass = iota
	SmartFertilizerCropFallback
	SmartFertilizerCropSmart
)

type SmartFertilizerCropIndex struct {
	byPlantID map[int]SmartFertilizerCropClass
	bySeedID  map[int]SmartFertilizerCropClass
}

func (index SmartFertilizerCropIndex) Classify(plantID int, seedID int) SmartFertilizerCropClass {
	if class, ok := index.byPlantID[plantID]; ok {
		return class
	}
	if class, ok := index.bySeedID[seedID]; ok {
		return class
	}
	return SmartFertilizerCropUnknown
}

type fruit struct {
	ID    int `json:"id"`
	Count int `json:"count"`
}

type itemInfoConfigItem struct {
	ID              int    `json:"id"`
	Type            int    `json:"type"`
	Name            string `json:"name"`
	InteractionType string `json:"interaction_type"`
	AssetName       string `json:"asset_name"`
	PriceID         int    `json:"price_id"`
	Price           int    `json:"price"`
	Jumps           string `json:"jumps"`
}

type levelCropMapping struct {
	Items []levelCropMappingItem `json:"items"`
}

type levelCropMappingItem struct {
	SeedID int `json:"seed_id"`
}

type activityCropMapping struct {
	Items []activityCropMeta `json:"items"`
}

type activityCropMeta struct {
	CropID        int    `json:"crop_id"`
	SeedID        int    `json:"seed_id"`
	FruitID       int    `json:"fruit_id"`
	Name          string `json:"name"`
	ImageCategory string `json:"image_category"`
}

type activityCropMetaIndex struct {
	byCropID  map[int]activityCropMeta
	bySeedID  map[int]activityCropMeta
	byFruitID map[int]activityCropMeta
	byName    map[string]activityCropMeta
}

func LoadCropAnalytics(root string) (CropAnalyticsPayload, error) {
	return LoadCropAnalyticsForLevel(root, CropAnalyticsOptions{})
}

func BuildStealCropOptions(root string) (StealCropOptionsPayload, error) {
	plants, itemMap, err := loadGameConfig(root)
	if err != nil {
		return StealCropOptionsPayload{OK: false, List: []StealCropOption{}, Error: err.Error()}, err
	}
	resolver := newLandImageResolver(root)
	cropAtlasIndex := loadCropAtlasMetaIndex(root)
	options := make([]StealCropOption, 0, len(plants))
	seen := map[int]bool{}
	for _, plant := range plants {
		name := strings.TrimSpace(plant.Name)
		if plant.ID <= 0 || name == "" || seen[plant.ID] {
			continue
		}
		seen[plant.ID] = true
		sortGroup := stealCropSortGroup(plant, name, itemMap, resolver.activityCrops)
		level := plant.LandLevelNeed
		if meta, found := cropAtlasIndex.byCropID[plant.ID]; found && meta.Level > 0 {
			level = meta.Level
		}
		options = append(options, StealCropOption{
			PlantID:   plant.ID,
			SeedID:    plant.SeedID,
			Name:      name,
			Level:     level,
			ImageURL:  resolver.resolvePlantMainImage(plant.SeedID, name),
			SortGroup: sortGroup,
			SortOrder: plant.ID,
		})
	}
	for _, meta := range cropAtlasIndex.byCropID {
		plant := cropAtlasPlantFromMeta(meta)
		name := strings.TrimSpace(plant.Name)
		if plant.ID <= 0 || name == "" || seen[plant.ID] {
			continue
		}
		seen[plant.ID] = true
		sortGroup := stealCropSortGroup(plant, name, itemMap, resolver.activityCrops)
		options = append(options, StealCropOption{
			PlantID:   plant.ID,
			SeedID:    plant.SeedID,
			Name:      name,
			Level:     meta.Level,
			ImageURL:  resolver.resolvePlantMainImage(plant.SeedID, name),
			SortGroup: sortGroup,
			SortOrder: meta.AtlasOrder,
		})
	}
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].SortGroup != options[j].SortGroup {
			return options[i].SortGroup < options[j].SortGroup
		}
		if options[i].Level != options[j].Level {
			return options[i].Level < options[j].Level
		}
		return options[i].PlantID < options[j].PlantID
	})
	return StealCropOptionsPayload{OK: true, UpdatedAt: time.Now().UnixMilli(), List: options}, nil
}

func stealCropSortGroup(plant plantConfigItem, name string, itemMap map[int]itemInfoConfigItem, activities activityCropMetaIndex) int {
	if activities.find(plant.ID, plant.SeedID, plant.Fruit.ID, name) != nil {
		return 2
	}
	if isShopEligiblePlant(plant, itemMap) {
		return 0
	}
	return 1
}

func BuildBackpackSeedOptions(seedList []any, request BackpackSeedOptionsRequest) BackpackSeedOptionsPayload {
	plants, itemMap, err := loadGameConfig(DefaultGameConfigRoot())
	if err != nil {
		return BackpackSeedOptionsPayload{
			OK:    false,
			List:  []BackpackSeedOption{},
			Error: err.Error(),
		}
	}
	plantBySeedID := buildPlantLookupBySeedID(plants, itemMap)

	selected := normalizeBackpackSeedIDs(request.SelectedSeedIDs)
	disabled := normalizeBackpackSeedIDs(request.DisabledSeedIDs)
	selectedIndex := make(map[int]int, len(selected))
	for index, seedID := range selected {
		selectedIndex[seedID] = index
	}
	disabledSet := make(map[int]bool, len(disabled))
	for _, seedID := range disabled {
		disabledSet[seedID] = true
	}

	optionsBySeedID := map[int]BackpackSeedOption{}
	for _, raw := range seedList {
		item := mapFromAny(raw)
		seedID := firstPositiveInt(
			intFromMap(item, "seedId"),
			intFromMap(item, "id"),
			intFromMap(item, "itemId"),
			intFromMap(item, "seed_id"),
		)
		count := firstNonNegativeInt(
			intFromMap(item, "count"),
			intFromMap(item, "stock"),
			intFromMap(item, "inventory"),
			intFromMap(item, "quantity"),
		)
		if seedID <= 0 || count <= 0 {
			continue
		}
		if !isPlantableBackpackSeedItem(seedID, item, itemMap, plantBySeedID) {
			continue
		}
		option := buildBackpackSeedOption(seedID, count, item, plantBySeedID, request.FourGridPlantEnabled)
		option.Disabled = disabledSet[seedID]
		optionsBySeedID[seedID] = option
	}

	if request.ForcePriority {
		for _, seedID := range selected {
			if _, exists := optionsBySeedID[seedID]; exists {
				continue
			}
			if !isPlantableBackpackSeedID(seedID, itemMap, plantBySeedID) {
				continue
			}
			option := buildBackpackSeedOption(seedID, 0, nil, plantBySeedID, request.FourGridPlantEnabled)
			option.Disabled = disabledSet[seedID]
			option.InBackpack = false
			option.RetainedByForcePriority = true
			optionsBySeedID[seedID] = option
		}
	}

	options := make([]BackpackSeedOption, 0, len(optionsBySeedID))
	for _, option := range optionsBySeedID {
		options = append(options, option)
	}
	sort.SliceStable(options, func(i, j int) bool {
		leftIndex, leftSelected := selectedIndex[options[i].SeedID]
		rightIndex, rightSelected := selectedIndex[options[j].SeedID]
		if leftSelected != rightSelected {
			return leftSelected
		}
		if leftSelected && rightSelected && leftIndex != rightIndex {
			return leftIndex < rightIndex
		}
		if options[i].RetainedByForcePriority != options[j].RetainedByForcePriority {
			return options[i].RetainedByForcePriority
		}
		if options[i].Plantable != options[j].Plantable {
			return options[i].Plantable
		}
		leftLevel := optionalIntValue(options[i].Level)
		rightLevel := optionalIntValue(options[j].Level)
		if leftLevel != rightLevel {
			return leftLevel > rightLevel
		}
		return strings.Compare(options[i].Name, options[j].Name) < 0
	})

	return BackpackSeedOptionsPayload{
		OK:        true,
		UpdatedAt: time.Now().UnixMilli(),
		List:      options,
	}
}

// isPlantableBackpackSeedItem accepts static plant seeds and runtime-verified
// seeds returned by gameCtl.getSeedList. Force-priority retention intentionally
// continues to use the static ID-only check below.
func isPlantableBackpackSeedItem(seedID int, item map[string]any, itemMap map[int]itemInfoConfigItem, plantBySeedID map[int]plantConfigItem) bool {
	if isPlantableBackpackSeedID(seedID, itemMap, plantBySeedID) {
		return true
	}
	return intFromMap(item, "type") == 5 && strings.EqualFold(
		strings.TrimSpace(firstNonEmptyString(
			stringFromMap(item, "interactionType"),
			stringFromMap(item, "interaction_type"),
		)),
		"plant",
	)
}

// isPlantableBackpackSeedID keeps only real plant seeds in the backpack seed selector.
// Known non-seeds (dog food, gift packs, decorations, etc.) must not enter the list even if
// the runtime bag snapshot mislabels them.
func isPlantableBackpackSeedID(seedID int, itemMap map[int]itemInfoConfigItem, plantBySeedID map[int]plantConfigItem) bool {
	if seedID <= 0 {
		return false
	}
	if _, ok := plantBySeedID[seedID]; ok {
		return true
	}
	info, ok := itemMap[seedID]
	if !ok {
		return false
	}
	return info.Type == 5 && strings.TrimSpace(info.InteractionType) == "plant"
}

func buildPlantLookupBySeedID(plants []plantConfigItem, itemMap map[int]itemInfoConfigItem) map[int]plantConfigItem {
	plantBySeedID := make(map[int]plantConfigItem, len(plants))
	plantByName := make(map[string]plantConfigItem, len(plants))
	plantAssetByName := make(map[string]plantConfigItem, len(plants))
	for _, plant := range plants {
		if plant.SeedID > 0 {
			plantBySeedID[plant.SeedID] = plant
		}
		if name := normalizedSeedPlantName(plant.Name); name != "" {
			plantByName[name] = plant
		}
	}
	for _, plant := range plants {
		if plant.SeedID <= 0 {
			continue
		}
		seedInfo, ok := itemMap[plant.SeedID]
		if !ok {
			continue
		}
		if asset := normalizedLookupText(seedInfo.AssetName); asset != "" {
			plantAssetByName[asset] = plant
		}
	}
	for _, item := range itemMap {
		if item.ID <= 0 || item.Type != 5 || strings.TrimSpace(item.InteractionType) != "plant" {
			continue
		}
		if _, exists := plantBySeedID[item.ID]; exists {
			continue
		}
		if plant, ok := plantByName[normalizedSeedPlantName(item.Name)]; ok {
			plantBySeedID[item.ID] = plant
			continue
		}
		if plant, ok := plantAssetByName[normalizedLookupText(item.AssetName)]; ok {
			plantBySeedID[item.ID] = plant
		}
	}
	return plantBySeedID
}

func normalizedSeedPlantName(name string) string {
	text := strings.TrimSpace(name)
	text = strings.TrimSuffix(text, "种子")
	return normalizedLookupText(text)
}

func buildBackpackSeedOption(seedID int, count int, item map[string]any, plantBySeedID map[int]plantConfigItem, fourGridPlantEnabled bool) BackpackSeedOption {
	plant, hasPlant := plantBySeedID[seedID]
	name := strings.TrimSuffix(firstNonEmptyString(stringFromMap(item, "name"), stringFromMap(item, "seedName")), "种子")
	if hasPlant && strings.TrimSpace(plant.Name) != "" {
		name = strings.TrimSpace(plant.Name)
	}
	if name == "" {
		name = fmt.Sprintf("seed=%d", seedID)
	}

	var level *int
	if runtimeLevel := firstPositiveInt(
		intFromMap(item, "level"),
		intFromMap(item, "seedLevel"),
		intFromMap(item, "seed_level"),
		intFromMap(item, "requiredLevel"),
		intFromMap(item, "unlockLevel"),
		intFromMap(item, "needLevel"),
		intFromMap(item, "minLevel"),
		intFromMap(item, "plantLevel"),
		intFromMap(item, "landLevelNeed"),
		intFromMap(item, "land_level_need"),
	); runtimeLevel > 0 {
		level = &runtimeLevel
	} else if hasPlant && plant.LandLevelNeed > 0 {
		levelValue := plant.LandLevelNeed
		level = &levelValue
	}

	plantSize := 1
	if hasPlant && plant.Size > 1 {
		plantSize = plant.Size
	}
	plantable := true
	plantableReason := ""
	plantableMessage := ""
	if plantSize >= 2 && !fourGridPlantEnabled {
		plantable = false
		plantableReason = "multi_tile_seed_not_supported"
		plantableMessage = fmt.Sprintf("%s为四格作物，当前背包种植策略不支持", name)
	}

	return BackpackSeedOption{
		SeedID:           seedID,
		Name:             name,
		Level:            level,
		Count:            count,
		BackpackCount:    count,
		InBackpack:       count > 0,
		Plantable:        plantable,
		PlantableReason:  plantableReason,
		PlantableMessage: plantableMessage,
		PlantSize:        plantSize,
	}
}

func normalizeBackpackSeedIDs(values []int) []int {
	seen := map[int]bool{}
	result := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func firstNonNegativeInt(values ...int) int {
	for _, value := range values {
		if value >= 0 {
			return value
		}
	}
	return 0
}

func optionalIntValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

type CropAnalyticsOptions struct {
	Sort              string
	RequestedMaxLevel int
	Profile           map[string]any
	SeedList          []any
	ShopList          []any
	RuntimeError      string
}

func LoadCropAnalyticsForLevel(root string, opts CropAnalyticsOptions) (CropAnalyticsPayload, error) {
	plants, itemMap, err := loadGameConfig(root)
	if err != nil {
		return CropAnalyticsPayload{}, err
	}
	canonicalSeedOrder, err := loadCanonicalLevelCropSeedOrder(root)
	if err != nil {
		return CropAnalyticsPayload{}, err
	}
	sortKey := strings.TrimSpace(strings.ToLower(opts.Sort))
	if sortKey == "" {
		sortKey = "exp"
	}
	levelInfo := NormalizeAnalyticsLevelRequest(opts.RequestedMaxLevel, opts.Profile)

	items := make([]CropAnalyticsItem, 0, len(plants))
	for _, plant := range plants {
		if plant.SeedID <= 0 || strings.TrimSpace(plant.GrowPhases) == "" {
			continue
		}
		canonicalOrder, isCanonical := canonicalSeedOrder[plant.SeedID]
		if !isShopEligiblePlant(plant, itemMap) || !isCanonical {
			continue
		}
		item, ok := cropAnalyticsItem(plant, itemMap)
		if ok {
			item.canonicalOrder = canonicalOrder
			if levelInfo.EffectiveMaxLevel > 0 && item.Level != nil && *item.Level > levelInfo.EffectiveMaxLevel {
				continue
			}
			items = append(items, item)
		}
	}
	sortCropAnalyticsItems(items, sortKey)
	strategies := plantStrategyModes()

	return CropAnalyticsPayload{
		Source:            sourceLabel(root),
		Items:             items,
		Sort:              sortKey,
		RequestedMaxLevel: levelInfo.RequestedMaxLevel,
		EffectiveMaxLevel: levelInfo.EffectiveMaxLevel,
		LevelSource:       levelInfo.LevelSource,
		Profile:           opts.Profile,
		Strategies:        strategies,
		Recommendations:   buildRecommendationCards(items, strategies, opts.SeedList, opts.ShopList),
		RuntimeError:      opts.RuntimeError,
	}, nil
}

type AnalyticsLevelInfo struct {
	RequestedMaxLevel int    `json:"requestedMaxLevel"`
	EffectiveMaxLevel int    `json:"effectiveMaxLevel"`
	LevelSource       string `json:"levelSource"`
}

func NormalizeAnalyticsLevelRequest(requestedLevel int, profile map[string]any) AnalyticsLevelInfo {
	if requestedLevel > 0 {
		return AnalyticsLevelInfo{RequestedMaxLevel: requestedLevel, EffectiveMaxLevel: requestedLevel, LevelSource: "config"}
	}
	for _, key := range []string{"plantLevel", "farmMaxLandLevel", "level"} {
		level := intFromMap(profile, key)
		if level > 0 {
			return AnalyticsLevelInfo{RequestedMaxLevel: 0, EffectiveMaxLevel: level, LevelSource: "profile"}
		}
	}
	return AnalyticsLevelInfo{RequestedMaxLevel: 0, EffectiveMaxLevel: 0, LevelSource: "none"}
}

func LoadAtlasPreview(root string) (AtlasPreviewPayload, error) {
	plants, _, err := loadGameConfig(root)
	if err != nil {
		return AtlasPreviewPayload{}, err
	}

	items := make([]AtlasItem, 0, len(plants))
	for _, plant := range plants {
		if plant.ID <= 0 || strings.TrimSpace(plant.Name) == "" || plant.SeedID <= 0 || plant.Fruit.ID <= 0 {
			continue
		}
		seasons := plant.Seasons
		if seasons <= 0 {
			seasons = 1
		}
		items = append(items, AtlasItem{
			ID:       plant.ID,
			Name:     strings.TrimSpace(plant.Name),
			SeedID:   plant.SeedID,
			FruitID:  plant.Fruit.ID,
			Level:    plant.LandLevelNeed,
			Seasons:  seasons,
			GrowTime: parseGrowTime(plant.GrowPhases, seasons),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Level != items[j].Level {
			return items[i].Level < items[j].Level
		}
		return items[i].ID < items[j].ID
	})

	return AtlasPreviewPayload{
		Source:         sourceLabel(root),
		Status:         "static_preview",
		Message:        "图鉴运行时刷新和购买待迁移，当前展示本地作物配置预览。",
		RefreshEnabled: false,
		BuyEnabled:     false,
		Sections: []AtlasSection{
			{ID: "crop", Label: "作物图鉴", Items: items, Total: len(items), Summary: summarizeAtlasItems(items)},
		},
	}, nil
}

func LandDetailsGate() LandDetailsPayload {
	reason := "土地详情需要迁移 /api/lands 与真实游戏运行时读取命令。"
	return LandDetailsPayload{
		Status:  "not_migrated",
		Message: reason,
		Lands:   []LandDetailsItem{},
		Actions: []RuntimeActionGate{
			{ID: "rush", Label: "一键催熟", Enabled: false, Reason: reason},
			{ID: "fertilize_normal", Label: "一键无机肥", Enabled: false, Reason: reason},
			{ID: "fertilize_organic", Label: "一键有机肥", Enabled: false, Reason: reason},
		},
	}
}

func WarehouseGate() WarehousePayload {
	reason := "仓库需要迁移 /api/warehouse、刷新和出售运行时命令。"
	return WarehousePayload{
		Status:  "not_migrated",
		Message: reason,
		Items:   []WarehouseItem{},
		Actions: []RuntimeActionGate{
			{ID: "refresh", Label: "刷新仓库", Enabled: false, Reason: reason},
			{ID: "sell", Label: "出售选中", Enabled: false, Reason: reason},
		},
	}
}

func BuildRuntimeLandDetails(status map[string]any) LandDetailsPayload {
	return BuildRuntimeLandDetailsForRoot(status, DefaultGameConfigRoot())
}

func BuildRuntimeLandDetailsForRoot(status map[string]any, root string) LandDetailsPayload {
	resolver := newLandImageResolver(root)
	grids := anySlice(status["grids"])
	lands := make([]LandDetailsItem, 0, len(grids))
	for index, raw := range grids {
		item := anyMap(raw)
		landID := intFromMap(item, "landId")
		if landID <= 0 {
			landID = index + 1
		}
		stage := strings.TrimSpace(strings.ToLower(firstNonEmptyString(stringFromMap(item, "stageKind"), stringFromMap(item, "status"))))
		plantName := stringFromMap(item, "plantName")
		displayPlantName := firstNonEmptyString(stringFromMap(item, "displayPlantName"), plantName)
		imageLookupName := firstNonEmptyString(plantName, displayPlantName)
		if stage == "" {
			if plantName != "" || boolFromMap(item, "hasPlant") || boolFromMap(item, "canHarvest") {
				stage = "unknown"
			} else {
				stage = "empty"
			}
		}
		if boolFromMap(item, "canHarvest") {
			stage = "mature"
		}
		plantID := firstPositiveInt(
			intFromMap(item, "plantId"),
			intFromMap(anyMap(anyMap(item["raw"])["plantData"]), "id"),
		)
		seedID := firstPositiveInt(
			intFromMap(item, "seedId"),
			intFromMap(anyMap(anyMap(item["raw"])["plantData"]), "seed_id"),
			intFromMap(anyMap(anyMap(item["raw"])["plantData"]), "seedId"),
			intFromMap(anyMap(anyMap(item["raw"])["config"]), "seed_id"),
			intFromMap(anyMap(anyMap(item["raw"])["config"]), "seedId"),
		)
		plant := resolver.plantByRuntimeIdentity(plantID, seedID, imageLookupName)
		if plantName == "" && plant != nil {
			plantName = strings.TrimSpace(plant.Name)
			displayPlantName = firstNonEmptyString(displayPlantName, plantName)
			imageLookupName = firstNonEmptyString(plantName, displayPlantName)
		}
		if seedID <= 0 && plant != nil {
			seedID = plant.SeedID
		}
		landType := stringFromMap(item, "landType")
		landTypeLabel := firstNonEmptyString(stringFromMap(item, "landTypeLabel"), landTypeDisplayLabel(landType))
		raw := anyMap(item["raw"])
		rawPlantData := anyMap(raw["plantData"])
		rawConfig := anyMap(raw["config"])
		currentSeason := firstPositiveInt(intFromMap(item, "currentSeason"), intFromMap(rawPlantData, "season"))
		totalSeason := firstPositiveInt(
			intFromMap(item, "totalSeason"),
			intFromMap(item, "seasons"),
			intFromMap(rawConfig, "seasons"),
			intFromMap(rawPlantData, "seasons"),
		)
		currentStage := intFromMap(item, "currentStage")
		phaseName := stringFromMap(item, "phaseName")
		landSize := firstPositiveInt(intFromMap(item, "landSize"), 1)
		occupancyPlantSize := firstPositiveInt(intFromMap(item, "occupancyPlantSize"), intFromMap(item, "plantSize"), landSize)
		mutation := resolver.resolveLandMutation(item, plantName, currentStage)
		imageURL := resolver.resolvePlantStageImage(seedID, imageLookupName, currentStage, phaseName)
		if runtimeImageURL := stringFromMap(item, "imageUrl"); isLocalGameImageURL(runtimeImageURL) {
			imageURL = runtimeImageURL
		}
		if mutation.ImageURL != "" {
			imageURL = mutation.ImageURL
			if mutation.DisplayName != "" {
				displayPlantName = mutation.DisplayName
			}
		}
		land := LandDetailsItem{
			ID:                       fmt.Sprintf("%d", landID),
			LandID:                   landID,
			PlantID:                  plantID,
			SeedID:                   seedID,
			LandType:                 landType,
			LandTypeLabel:            landTypeLabel,
			PlantName:                plantName,
			DisplayPlantName:         displayPlantName,
			ImageURL:                 imageURL,
			Status:                   stage,
			StatusLabel:              landStatusLabel(stage),
			MatureAtMs:               int64FromMap(item, "matureAtMs"),
			CurrentSeason:            currentSeason,
			TotalSeason:              totalSeason,
			CurrentStage:             currentStage,
			PhaseName:                phaseName,
			IsMultiSeason:            boolFromMap(item, "isMultiSeason") || totalSeason > 1,
			LandSize:                 landSize,
			OccupancyPlantSize:       occupancyPlantSize,
			OccupancyAnchorLandID:    intFromMap(item, "occupancyAnchorLandId"),
			OccupiedByMultiTilePlant: boolFromMap(item, "occupiedByMultiTilePlant"),
			HasMutation:              mutation.HasMutation,
			MutationLabel:            mutation.Label,
			MutationIconURL:          mutation.IconURL,
			MutationImageURL:         mutation.ImageURL,
			MutationTypes:            mutation.Types,
			NeedWater:                boolFromMap(item, "needWater") || boolFromMap(item, "needsWater"),
			NeedWeed:                 boolFromMap(item, "needWeed") || boolFromMap(item, "needsEraseGrass"),
			NeedBug:                  boolFromMap(item, "needBug") || boolFromMap(item, "needsKillBug"),
			NeedGoldenBug:            boolFromMap(item, "needGoldenBug") || boolFromMap(item, "needsGoldenBug") || boolFromMap(item, "hasGoldenBug"),
			NeedEraseDead:            boolFromMap(item, "needEraseDead") || boolFromMap(item, "needsEraseDead"),
			CanHarvest:               boolFromMap(item, "canHarvest"),
		}
		if level := intFromMap(item, "landLevel"); level > 0 {
			land.LandLevel = &level
		}
		if matureInSec, ok := optionalIntFromMap(item, "matureInSec"); ok {
			land.MatureInSec = &matureInSec
			if land.MatureAtMs <= 0 && matureInSec > 0 {
				land.MatureAtMs = time.Now().Add(time.Duration(matureInSec) * time.Second).UnixMilli()
			}
		}
		land.MatureEtaText = firstNonEmptyString(stringFromMap(item, "matureEtaText"), landMatureEtaText(stage, land.MatureInSec))
		lands = append(lands, land)
	}
	sort.SliceStable(lands, func(i, j int) bool {
		return lands[i].LandID < lands[j].LandID
	})
	return LandDetailsPayload{
		Status:     "runtime",
		Message:    "土地详情已从游戏运行时读取。",
		FarmType:   stringFromMap(status, "farmType"),
		TotalGrids: firstPositiveInt(intFromMap(status, "totalGrids"), len(lands)),
		Lands:      lands,
		Actions: []RuntimeActionGate{
			{ID: "rush", Label: "一键催熟", Enabled: true},
			{ID: "fertilize_normal", Label: "一键无机肥", Enabled: true},
			{ID: "fertilize_organic", Label: "一键有机肥", Enabled: true},
		},
	}
}

func BuildRuntimeWarehouse(snapshot map[string]any, itemMap map[int]itemInfoConfigItem) WarehousePayload {
	itemsRaw := anySlice(snapshot["items"])
	imageResolver := newLandImageResolver(DefaultGameConfigRoot())
	items := make([]WarehouseItem, 0, len(itemsRaw))
	for index, raw := range itemsRaw {
		item := anyMap(raw)
		itemID := firstPositiveInt(
			intFromMap(item, "itemId"),
			intFromMap(item, "id"),
			intFromMap(item, "cfgId"),
		)
		count := firstPositiveInt(
			intFromMap(item, "count"),
			intFromMap(item, "num"),
			intFromMap(item, "value"),
		)
		if itemID <= 0 || count <= 0 {
			continue
		}
		info := itemMap[itemID]
		name := stringFromMap(item, "name")
		if name == "" {
			name = strings.TrimSpace(info.Name)
		}
		if name == "" {
			name = fmt.Sprintf("物品 %d", itemID)
		}
		category := warehouseCategory(itemID, info)
		locked := boolFromMap(item, "locked") || boolFromMap(item, "isLocked") || boolFromMap(item, "lock")
		saleUnitPrice := warehouseSaleUnitPrice(item)
		runtimeCanSell, hasRuntimeCanSell := optionalBoolFromMap(item, "canSell")
		canSell := !locked && saleUnitPrice > 0
		if hasRuntimeCanSell {
			canSell = !locked && runtimeCanSell
		}
		warehouseKey := stringFromMap(item, "warehouseKey")
		if warehouseKey == "" {
			warehouseKey = fmt.Sprintf("%d:%d", itemID, index)
		}
		items = append(items, WarehouseItem{
			ID:                 warehouseKey,
			ItemID:             itemID,
			Name:               name,
			Count:              count,
			Category:           category,
			CategoryLabel:      warehouseCategoryLabel(category),
			ImageURL:           resolveWarehouseItemImageURL(imageResolver, itemID, name, category),
			CanSell:            canSell,
			Locked:             locked,
			EstimatedSellPrice: saleUnitPrice * count,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CanSell != items[j].CanSell {
			return items[i].CanSell
		}
		if items[i].EstimatedSellPrice != items[j].EstimatedSellPrice {
			return items[i].EstimatedSellPrice > items[j].EstimatedSellPrice
		}
		return items[i].ItemID < items[j].ItemID
	})
	return WarehousePayload{
		Status:  "runtime",
		Message: "仓库已从游戏运行时读取。",
		Items:   items,
		Actions: []RuntimeActionGate{
			{ID: "refresh", Label: "刷新仓库", Enabled: true},
			{ID: "sell", Label: "出售选中", Enabled: true},
		},
		Summary: summarizeWarehousePayload(items),
	}
}

type landImageResolver struct {
	root          string
	byID          map[int]plantConfigItem
	bySeedID      map[int]plantConfigItem
	byFruitID     map[int]plantConfigItem
	byPlantName   map[string]plantConfigItem
	activityCrops activityCropMetaIndex
}

type cropAtlasMeta struct {
	ID            int
	Name          string
	SeedID        int
	FruitID       int
	Level         int
	LandLevelNeed int
	Seasons       int
	GrowPhases    string
	Exp           int
	AtlasOrder    int
}

type cropAtlasMetaIndex struct {
	byName    map[string]cropAtlasMeta
	bySeedID  map[int]cropAtlasMeta
	byFruitID map[int]cropAtlasMeta
	byCropID  map[int]cropAtlasMeta
}

type mutationAtlasMeta struct {
	Name       string
	GroupName  string
	ItemChips  string
	ItemStats  string
	ImagePath  string
	AtlasOrder int
	AtlasPoint int
}

type mutationAtlasMetaIndex struct {
	byName  map[string]mutationAtlasMeta
	byAsset map[string]mutationAtlasMeta
}

type landMutationSummary struct {
	HasMutation bool
	Label       string
	IconURL     string
	ImageURL    string
	DisplayName string
	Types       []LandMutationTypeItem
}

type landStageImageEntry struct {
	Index int
	Label string
	Path  string
}

var mutationRuntimeTypeNames = map[int]string{
	1:  "冰冻",
	2:  "爱心",
	3:  "暗化",
	4:  "湿润",
	5:  "黄金",
	6:  "哈哈",
	7:  "塔塔",
	8:  "荷华",
	9:  "月华",
	10: "绵绵",
	11: "梦蝶",
}

var mutationTypeIDsByName = map[string]int{
	"冰冻": 1,
	"爱心": 2,
	"暗化": 3,
	"湿润": 4,
	"黄金": 5,
	"哈哈": 6,
	"塔塔": 7,
	"荷华": 8,
	"月华": 9,
	"绵绵": 10,
	"梦蝶": 11,
}

var mutationDisplayOrder = []string{"月华", "荷华", "绵绵", "塔塔", "哈哈", "黄金", "冰冻", "爱心", "暗化", "湿润", "梦蝶"}

var mutationPlantNamesByRuntimeID = map[int]string{
	1020804: "绵绵糖果",
	1120167: "黄金·欢乐糖果",
	1049001: "黄金·哈哈小南瓜",
	204002:  "哈哈南瓜塔",
	204003:  "黄金·哈哈南瓜塔",
	204004:  "月华宝荷",
	204005:  "黄金·月华宝荷",
	1128003: "黄金·蝶梦星铃",
}

var mutationPlantNamesByTypeName = map[string]string{
	"哈哈": "哈哈小南瓜",
	"塔塔": "哈哈南瓜塔",
	"荷华": "荷花",
	"月华": "月华宝荷",
	"绵绵": "绵绵糖果",
}

func newLandImageResolver(root string) landImageResolver {
	if strings.TrimSpace(root) == "" {
		root = DefaultGameConfigRoot()
	}
	resolver := landImageResolver{
		root:          root,
		byID:          map[int]plantConfigItem{},
		bySeedID:      map[int]plantConfigItem{},
		byFruitID:     map[int]plantConfigItem{},
		byPlantName:   map[string]plantConfigItem{},
		activityCrops: loadActivityCropMetaIndex(root),
	}
	var plants []plantConfigItem
	if err := readGameConfigJSON(root, "Plant.json", &plants); err != nil {
		return resolver
	}
	for _, plant := range plants {
		if plant.ID > 0 {
			resolver.byID[plant.ID] = plant
		}
		if plant.SeedID > 0 {
			resolver.bySeedID[plant.SeedID] = plant
		}
		if plant.Fruit.ID > 0 {
			resolver.byFruitID[plant.Fruit.ID] = plant
		}
		name := normalizedLookupText(plant.Name)
		if name != "" {
			resolver.byPlantName[name] = plant
		}
	}
	resolver.mergeCropAtlasFallbacks(loadCropAtlasMetaIndex(root))
	return resolver
}

func loadActivityCropMetaIndex(root string) activityCropMetaIndex {
	index := activityCropMetaIndex{
		byCropID:  map[int]activityCropMeta{},
		bySeedID:  map[int]activityCropMeta{},
		byFruitID: map[int]activityCropMeta{},
		byName:    map[string]activityCropMeta{},
	}
	var mapping activityCropMapping
	if err := readGameConfigJSON(root, filepath.Join("plant_images", "stages", "_mappings", "activity_crops.json"), &mapping); err != nil {
		return index
	}
	for _, meta := range mapping.Items {
		meta.Name = strings.TrimSpace(meta.Name)
		meta.ImageCategory = strings.TrimSpace(meta.ImageCategory)
		if meta.Name == "" || meta.ImageCategory == "" {
			continue
		}
		if meta.CropID > 0 {
			index.byCropID[meta.CropID] = meta
		}
		if meta.SeedID > 0 {
			index.bySeedID[meta.SeedID] = meta
		}
		if meta.FruitID > 0 {
			index.byFruitID[meta.FruitID] = meta
		}
		index.byName[normalizedLookupText(meta.Name)] = meta
	}
	return index
}

func (index activityCropMetaIndex) find(cropID int, seedID int, fruitID int, name string) *activityCropMeta {
	for _, id := range []int{cropID, seedID, fruitID} {
		if id <= 0 {
			continue
		}
		if meta, ok := index.byCropID[id]; ok {
			return &meta
		}
		if meta, ok := index.bySeedID[id]; ok {
			return &meta
		}
		if meta, ok := index.byFruitID[id]; ok {
			return &meta
		}
	}
	for _, candidate := range collectUniqueText(name, stripGoldCropNameGo(name)) {
		if meta, ok := index.byName[normalizedLookupText(candidate)]; ok {
			return &meta
		}
	}
	return nil
}

func (r landImageResolver) plantByRuntimeIdentity(plantID int, seedID int, plantName string) *plantConfigItem {
	if plantID > 0 {
		if plant, ok := r.byID[plantID]; ok {
			return &plant
		}
		if plant, ok := r.bySeedID[plantID]; ok {
			return &plant
		}
		if plant, ok := r.byFruitID[plantID]; ok {
			return &plant
		}
	}
	if seedID > 0 {
		if plant, ok := r.bySeedID[seedID]; ok {
			return &plant
		}
	}
	if plantName != "" {
		if plant, ok := r.byPlantName[normalizedLookupText(plantName)]; ok {
			return &plant
		}
	}
	return nil
}

func (r landImageResolver) plantByCropIdentity(fruitID int, seedID int, plantName string) *plantConfigItem {
	if fruitID > 0 {
		if plant, ok := r.byFruitID[fruitID]; ok {
			return &plant
		}
	}
	if seedID > 0 {
		if plant, ok := r.bySeedID[seedID]; ok {
			return &plant
		}
	}
	if plantName != "" {
		if plant, ok := r.byPlantName[normalizedLookupText(plantName)]; ok {
			return &plant
		}
	}
	if fruitID > 0 {
		if plant, ok := r.byID[fruitID]; ok {
			return &plant
		}
	}
	return nil
}

func (r landImageResolver) mergeCropAtlasFallbacks(index cropAtlasMetaIndex) {
	for _, meta := range index.byFruitID {
		plant := cropAtlasPlantFromMeta(meta)
		if plant.Name == "" {
			continue
		}
		if plant.ID > 0 {
			if _, exists := r.byID[plant.ID]; !exists {
				r.byID[plant.ID] = plant
			}
		}
		if plant.SeedID > 0 {
			if _, exists := r.bySeedID[plant.SeedID]; !exists {
				r.bySeedID[plant.SeedID] = plant
			}
		}
		if plant.Fruit.ID > 0 {
			if _, exists := r.byFruitID[plant.Fruit.ID]; !exists {
				r.byFruitID[plant.Fruit.ID] = plant
			}
		}
		name := normalizedLookupText(plant.Name)
		if name != "" {
			if _, exists := r.byPlantName[name]; !exists {
				r.byPlantName[name] = plant
			}
		}
	}
}

func cropAtlasPlantFromMeta(meta cropAtlasMeta) plantConfigItem {
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		return plantConfigItem{}
	}
	seasons := meta.Seasons
	if seasons <= 0 {
		seasons = 1
	}
	return plantConfigItem{
		ID:            meta.ID,
		Name:          name,
		Fruit:         fruit{ID: meta.FruitID},
		SeedID:        meta.SeedID,
		LandLevelNeed: firstPositiveInt(meta.LandLevelNeed, meta.Level),
		Seasons:       seasons,
		GrowPhases:    meta.GrowPhases,
		Exp:           meta.Exp,
	}
}

func (r landImageResolver) resolvePlantStageImage(seedID int, plantName string, currentStage int, phaseName string) string {
	path := r.resolvePlantStageImagePath(seedID, plantName, currentStage, phaseName)
	return gameConfigLocalImageURL(r.root, path)
}

func (r landImageResolver) resolvePlantStageImagePath(seedID int, plantName string, currentStage int, phaseName string) string {
	plant := r.plantByRuntimeIdentity(0, seedID, plantName)
	names := collectUniqueText(plantName)
	if plant != nil {
		names = appendUniqueText(names, plant.Name)
	}
	for _, name := range names {
		stageEntries := readStageEntries(filepath.Join(r.root, "plant_images", "stages", "作物", name))
		if len(stageEntries) == 0 {
			continue
		}
		if path := plantStageImagePath(stageEntries, plant, currentStage, phaseName); path != "" {
			return path
		}
		if path := primaryStageImagePath(stageEntries); path != "" {
			return path
		}
	}
	if meta := r.activityCrops.find(plantIDOf(plant), seedID, fruitIDOf(plant), plantName); meta != nil {
		for _, assetName := range activityCropAssetNames(*meta, append(names, plantName)...) {
			entries := readStageEntriesForCrop(
				filepath.Join(r.root, "plant_images", "stages", meta.ImageCategory, meta.Name),
				assetName,
			)
			if path := plantStageImagePath(entries, plant, currentStage, phaseName); path != "" {
				return path
			}
		}
		if requestedPlantStageIsSeed(currentStage, phaseName) {
			if path := r.genericSeedStageImagePath(); path != "" {
				return path
			}
		}
		if path := r.resolveActivityCropMainImagePath(*meta, append(names, plantName)...); path != "" {
			return path
		}
	}
	if requestedPlantStageIsSeed(currentStage, phaseName) {
		if path := r.genericSeedStageImagePath(); path != "" {
			return path
		}
	}
	return imageFirstExistingPath([]string{
		filepath.Join(r.root, "plant_images", "default", "400.png"),
		filepath.Join(r.root, "plant_images", "default", "400.jpg"),
		filepath.Join(r.root, "plant_images", "default", "400.jpeg"),
	})
}

func plantStageImagePath(entries []landStageImageEntry, plant *plantConfigItem, currentStage int, phaseName string) string {
	if phaseName != "" {
		for _, entry := range entries {
			if normalizedLookupText(entry.Label) == normalizedLookupText(phaseName) {
				return entry.Path
			}
		}
		if plant != nil {
			phaseIndex := growPhaseIndexByName(plant.GrowPhases, phaseName)
			if phaseIndex > 0 {
				for _, entry := range entries {
					if entry.Index == phaseIndex {
						return entry.Path
					}
				}
			}
		}
	}
	if currentStage > 0 {
		for _, entry := range entries {
			if entry.Index == currentStage {
				return entry.Path
			}
		}
	}
	return ""
}

func requestedPlantStageIsSeed(currentStage int, phaseName string) bool {
	return currentStage == 1 || normalizedLookupText(phaseName) == normalizedLookupText("种子")
}

func plantIDOf(plant *plantConfigItem) int {
	if plant == nil {
		return 0
	}
	return plant.ID
}

func fruitIDOf(plant *plantConfigItem) int {
	if plant == nil {
		return 0
	}
	return plant.Fruit.ID
}

func activityCropAssetNames(meta activityCropMeta, names ...string) []string {
	result := collectUniqueText(names...)
	return appendUniqueText(result, meta.Name)
}

func (r landImageResolver) resolveActivityCropMainImagePath(meta activityCropMeta, names ...string) string {
	dir := filepath.Join(r.root, "plant_images", "stages", meta.ImageCategory, meta.Name)
	for _, assetName := range activityCropAssetNames(meta, names...) {
		if path := primaryStageImagePath(readStageEntriesForCrop(dir, assetName)); path != "" {
			return path
		}
	}
	return ""
}

func (r landImageResolver) genericSeedStageImagePath() string {
	if path := imageFirstExistingPath([]string{
		filepath.Join(r.root, "plant_images", "stages", "作物", "白萝卜", "白萝卜_01_种子.png"),
	}); path != "" {
		return path
	}
	cropsDir := filepath.Join(r.root, "plant_images", "stages", "作物")
	dirs, err := os.ReadDir(cropsDir)
	if err != nil {
		return ""
	}
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		for _, entry := range readStageEntries(filepath.Join(cropsDir, dir.Name())) {
			if entry.Index == 1 || normalizedLookupText(entry.Label) == normalizedLookupText("种子") {
				return entry.Path
			}
		}
	}
	return ""
}

func (r landImageResolver) resolveLandMutation(item map[string]any, plantName string, currentStage int) landMutationSummary {
	rawMutation := anyMap(item["mutation"])
	typeNames := mutationTypeNamesFromRuntime(item, rawMutation)
	hasMutation := boolFromMap(rawMutation, "hasMutation") || len(typeNames) > 0 ||
		len(anySlice(rawMutation["plantItems"])) > 0 ||
		len(numberListFromAny(rawMutation["activeMutantPlantIds"])) > 0 ||
		len(numberListFromAny(rawMutation["mutantPlantIds"])) > 0
	if !hasMutation {
		return landMutationSummary{}
	}

	types := make([]LandMutationTypeItem, 0, len(typeNames))
	for _, name := range typeNames {
		types = append(types, LandMutationTypeItem{
			TypeID:  mutationTypeIDsByName[name],
			Name:    name,
			IconURL: r.resolveMutationIcon(name),
		})
	}
	if len(types) == 0 {
		types = append(types, LandMutationTypeItem{Name: "变异"})
	}
	labelNames := make([]string, 0, len(types))
	for _, item := range types {
		labelNames = append(labelNames, item.Name)
	}
	runtimeDisplayNames := r.mutationPlantNamesFromRuntime(rawMutation, plantName)
	displayName := r.pickMutationDisplayPlantName(rawMutation, plantName, labelNames, runtimeDisplayNames)
	imageURL := ""
	if displayName != "" {
		imageURL = r.resolveMutationPlantImage(displayName, currentStage)
	}
	if imageURL == "" {
		imageURL = r.resolveMutationPlantImage(firstNonEmptyString(displayName, plantName), currentStage)
	}
	return landMutationSummary{
		HasMutation: true,
		Label:       strings.Join(labelNames, "、"),
		IconURL:     types[0].IconURL,
		ImageURL:    imageURL,
		DisplayName: displayName,
		Types:       types,
	}
}

func (r landImageResolver) pickMutationDisplayPlantName(rawMutation map[string]any, plantName string, typeNames []string, runtimeDisplayNames []string) string {
	for _, raw := range anySlice(rawMutation["plantItems"]) {
		item := anyMap(raw)
		name := firstNonEmptyString(stringFromMap(item, "name"), stringFromMap(item, "plantName"), stringFromMap(item, "outputName"))
		if name != "" {
			return name
		}
	}
	for _, key := range []string{"displayPredictionText", "predictionText", "outputName"} {
		if name := stringFromMap(rawMutation, key); name != "" {
			return name
		}
	}
	for _, name := range runtimeDisplayNames {
		if strings.TrimSpace(name) != "" {
			return name
		}
	}
	for _, name := range typeNames {
		if outputName := mutationPlantNamesByTypeName[name]; outputName != "" {
			return outputName
		}
		if name == "黄金" && strings.TrimSpace(plantName) != "" {
			return "黄金·" + stripGoldCropNameGo(plantName)
		}
	}
	return ""
}

func (r landImageResolver) mutationPlantNamesFromRuntime(rawMutation map[string]any, plantName string) []string {
	runtimeIDs := collectMutationRuntimePlantIDs(rawMutation)
	if plant, ok := r.byPlantName[normalizedLookupText(plantName)]; ok {
		effectMap := parseMutationPlantEffectMap(plant.MutantEffectPlant)
		activeTypes := numberListFromAny(firstExistingAny(
			rawMutation["activeMutantTypes"],
			rawMutation["mutantTypes"],
			rawMutation["startedMutantIds"],
			rawMutation["lastHarvestMutantTypes"],
		))
		for _, typeID := range activeTypes {
			if runtimeID := effectMap[typeID]; runtimeID > 0 {
				runtimeIDs = append(runtimeIDs, runtimeID)
			}
		}
		if combinationKey := mutationCombinationKey(activeTypes); combinationKey != "" {
			if runtimeID := effectMap[combinationKey]; runtimeID > 0 {
				runtimeIDs = append(runtimeIDs, runtimeID)
			}
		}
	}
	names := collectUniqueText()
	for _, runtimeID := range runtimeIDs {
		if name := mutationPlantNameByRuntimeID(runtimeID); name != "" {
			names = appendUniqueText(names, name)
		}
	}
	return names
}

func collectMutationRuntimePlantIDs(rawMutation map[string]any) []int {
	result := []int{}
	for _, key := range []string{"activeMutantPlantIds", "mutantPlantIds", "startedMutantPlantIds"} {
		result = append(result, numberListFromAny(rawMutation[key])...)
	}
	return result
}

func parseMutationPlantEffectMap(value string) map[any]int {
	result := map[any]int{}
	for _, part := range strings.Split(value, ";") {
		pieces := strings.Split(part, ":")
		if len(pieces) < 2 {
			continue
		}
		typeKey := strings.TrimSpace(pieces[0])
		plantID := parseLeadingInt(pieces[1])
		if typeKey == "" || plantID <= 0 {
			continue
		}
		if _, ok := result[typeKey]; !ok {
			result[typeKey] = plantID
		}
		typeIDs := numberListFromMutationKey(typeKey)
		if len(typeIDs) > 0 {
			if _, ok := result[typeIDs[0]]; !ok {
				result[typeIDs[0]] = plantID
			}
		}
		if len(typeIDs) > 1 {
			if key := mutationCombinationKey(typeIDs); key != "" {
				if _, ok := result[key]; !ok {
					result[key] = plantID
				}
			}
		}
	}
	return result
}

func numberListFromMutationKey(value string) []int {
	result := []int{}
	for _, item := range strings.Split(value, "_") {
		id := parseLeadingInt(item)
		if id > 0 {
			result = append(result, id)
		}
	}
	return result
}

func mutationCombinationKey(typeIDs []int) string {
	values := append([]int(nil), typeIDs...)
	sort.Ints(values)
	parts := []string{}
	for _, id := range values {
		if id > 0 {
			parts = append(parts, strconv.Itoa(id))
		}
	}
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts, "_")
}

func mutationPlantNameByRuntimeID(runtimeID int) string {
	if name := mutationPlantNamesByRuntimeID[runtimeID]; name != "" {
		return name
	}
	assetID := normalizeMutationPlantAssetID(runtimeID)
	if name := mutationPlantNamesByRuntimeID[assetID]; name != "" {
		return name
	}
	return ""
}

func normalizeMutationPlantAssetID(value int) int {
	if value <= 0 {
		return 0
	}
	text := strconv.Itoa(value)
	if value >= 10000 && len(text) >= 4 {
		suffix, err := strconv.Atoi(text[len(text)-4:])
		if err == nil && suffix > 0 {
			return suffix
		}
	}
	return value
}

func loadCropAtlasMetaIndex(root string) cropAtlasMetaIndex {
	index := cropAtlasMetaIndex{
		byName:    map[string]cropAtlasMeta{},
		bySeedID:  map[int]cropAtlasMeta{},
		byFruitID: map[int]cropAtlasMeta{},
		byCropID:  map[int]cropAtlasMeta{},
	}
	mappingPath := filepath.Join(root, "plant_images", "stages", "_mappings", "crop_level_mapping.json")
	raw, err := os.ReadFile(mappingPath)
	if err != nil {
		return index
	}
	var parsed struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return index
	}
	for order, item := range parsed.Items {
		name := strings.TrimSpace(stringFromMap(item, "name"))
		if name == "" {
			continue
		}
		meta := cropAtlasMeta{
			ID:            intFromMap(item, "crop_id"),
			Name:          name,
			SeedID:        intFromMap(item, "seed_id"),
			FruitID:       intFromMap(item, "fruit_id"),
			Level:         intFromMap(item, "level"),
			LandLevelNeed: intFromMap(item, "land_level_need"),
			Seasons:       firstPositiveInt(intFromMap(item, "seasons"), 1),
			GrowPhases:    stringFromMap(item, "grow_phases"),
			Exp:           intFromMap(item, "exp"),
			AtlasOrder:    order + 1,
		}
		index.byName[normalizedLookupText(name)] = meta
		if meta.SeedID > 0 {
			index.bySeedID[meta.SeedID] = meta
		}
		if meta.FruitID > 0 {
			index.byFruitID[meta.FruitID] = meta
		}
		if meta.ID > 0 {
			index.byCropID[meta.ID] = meta
		}
	}
	return index
}

func (m cropAtlasMetaIndex) byRuntimeIdentity(fruitID int, seedID int, name string) cropAtlasMeta {
	if fruitID > 0 {
		if meta, ok := m.byFruitID[fruitID]; ok {
			return meta
		}
		if meta, ok := m.byCropID[fruitID]; ok {
			return meta
		}
	}
	if seedID > 0 {
		if meta, ok := m.bySeedID[seedID]; ok {
			return meta
		}
	}
	if name != "" {
		if meta, ok := m.byName[normalizedLookupText(name)]; ok {
			return meta
		}
	}
	return cropAtlasMeta{}
}

func loadMutationAtlasMetaIndex(root string) mutationAtlasMetaIndex {
	index := mutationAtlasMetaIndex{byName: map[string]mutationAtlasMeta{}, byAsset: map[string]mutationAtlasMeta{}}
	atlasOrder := 0
	for _, mappingPath := range []string{
		filepath.Join(root, "plant_images", "stages", "_mappings", "mutation_mapping.json"),
		filepath.Join(root, "plant_images", "stages", "变异", "mutation_mapping.json"),
	} {
		raw, err := os.ReadFile(mappingPath)
		if err != nil {
			continue
		}
		var parsed struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			continue
		}
		for _, item := range parsed.Items {
			if stringFromMap(item, "section_name") != "超变图鉴" {
				continue
			}
			name := firstNonEmptyString(stringFromMap(item, "item_name"), stringFromMap(item, "image_alt"))
			if name == "" {
				continue
			}
			role := stringFromMap(item, "image_role")
			existing := index.byName[normalizedLookupText(name)]
			localPath := mutationAtlasLocalImagePath(root, stringFromMap(item, "local_path"))
			assetID := 0
			toneKey := "normal"
			if role == "main" {
				atlasOrder++
				existing.AtlasOrder = firstPositiveInt(existing.AtlasOrder, atlasOrder)
				if localPath != "" {
					existing.ImagePath = localPath
				}
				assetID = mutationAtlasAssetIDFromPath(stringFromMap(item, "image_path"))
				if assetID > 0 {
					if strings.HasPrefix(name, "黄金·") {
						toneKey = "gold"
					}
				}
			} else if existing.ImagePath == "" && localPath != "" {
				existing.ImagePath = localPath
			}
			existing.Name = name
			existing.GroupName = firstNonEmptyString(stringFromMap(item, "group_name"), existing.GroupName)
			existing.ItemChips = firstNonEmptyString(stringFromMap(item, "item_chips"), existing.ItemChips)
			existing.ItemStats = firstNonEmptyString(stringFromMap(item, "item_stats"), existing.ItemStats)
			existing.AtlasPoint = firstPositiveInt(existing.AtlasPoint, parseAtlasPointText(existing.ItemChips), parseAtlasPointText(existing.ItemStats))
			index.byName[normalizedLookupText(name)] = existing
			if assetID > 0 {
				index.byAsset[fmt.Sprintf("%s:%d", toneKey, assetID)] = existing
				index.byAsset[strconv.Itoa(assetID)] = existing
			}
		}
	}
	return index
}

func mutationAtlasLocalImagePath(root string, localPath string) string {
	clean := strings.TrimSpace(localPath)
	if clean == "" {
		return ""
	}
	path := filepath.Join(root, "plant_images", "stages", "变异", filepath.FromSlash(clean))
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

func mutationAtlasAssetIDFromPath(imagePath string) int {
	match := regexp.MustCompile(`Crop_(\d+)`).FindStringSubmatch(imagePath)
	if len(match) < 2 {
		return 0
	}
	value, _ := strconv.Atoi(match[1])
	return value
}

func (m mutationAtlasMetaIndex) byRuntimeID(runtimeID int) mutationAtlasMeta {
	assetID := normalizeMutationPlantAssetID(runtimeID)
	if assetID <= 0 {
		return mutationAtlasMeta{}
	}
	if strings.HasPrefix(strconv.Itoa(runtimeID), "112") || strings.HasPrefix(strconv.Itoa(runtimeID), "104") {
		if meta, ok := m.byAsset[fmt.Sprintf("gold:%d", assetID)]; ok {
			return meta
		}
	}
	if strings.HasPrefix(strconv.Itoa(runtimeID), "102") {
		if meta, ok := m.byAsset[fmt.Sprintf("normal:%d", assetID)]; ok {
			return meta
		}
	}
	if meta, ok := m.byAsset[strconv.Itoa(assetID)]; ok {
		return meta
	}
	if name := mutationPlantNameByRuntimeID(runtimeID); name != "" {
		return m.byPlantName(name)
	}
	return mutationAtlasMeta{}
}

func (m mutationAtlasMetaIndex) byPlantName(name string) mutationAtlasMeta {
	return m.byName[normalizedLookupText(name)]
}

func parseAtlasPointText(value string) int {
	text := strings.TrimSpace(value)
	if text == "" {
		return 0
	}
	if match := regexp.MustCompile(`\+(\d+)`).FindStringSubmatch(text); len(match) > 1 {
		parsed, _ := strconv.Atoi(match[1])
		return parsed
	}
	if match := regexp.MustCompile(`图鉴点数[:：]\s*(\d+)`).FindStringSubmatch(text); len(match) > 1 {
		parsed, _ := strconv.Atoi(match[1])
		return parsed
	}
	return 0
}

func (r landImageResolver) resolveMutationIcon(typeName string) string {
	name := strings.TrimSpace(typeName)
	if name == "" {
		return ""
	}
	dir := filepath.Join(r.root, "plant_images", "stages", "变异", "变异宝典", name)
	return gameConfigLocalImageURL(r.root, imageFirstExistingPath([]string{
		filepath.Join(dir, name+"_00_变异图标.png"),
		filepath.Join(dir, strings.ToLower(name)+".png"),
		firstImageInDir(dir),
	}))
}

func (r landImageResolver) resolveMutationPlantImage(name string, currentStage int) string {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(r.root, "plant_images", "stages", "变异", "超变图鉴", "黄金果实", cleanName),
		filepath.Join(r.root, "plant_images", "stages", "变异", "超变图鉴", "活动果实", cleanName),
		filepath.Join(r.root, "plant_images", "stages", "变异", "超变图鉴", "装扮果实", cleanName),
		filepath.Join(r.root, "plant_images", "stages", "超变果实", cleanName),
	}
	for _, dir := range candidates {
		entries := readStageEntries(dir)
		if len(entries) == 0 {
			continue
		}
		if currentStage > 0 {
			for _, entry := range entries {
				if entry.Index == currentStage {
					return gameConfigLocalImageURL(r.root, entry.Path)
				}
			}
		}
		return gameConfigLocalImageURL(r.root, primaryStageImagePath(entries))
	}
	if meta := r.activityCrops.find(0, 0, 0, cleanName); meta != nil {
		if path := r.resolveActivityCropMainImagePath(*meta, cleanName); path != "" {
			return gameConfigLocalImageURL(r.root, path)
		}
	}
	return ""
}

func mutationTypeNamesFromRuntime(item map[string]any, rawMutation map[string]any) []string {
	names := collectUniqueText()
	for _, raw := range anySlice(rawMutation["typeItems"]) {
		typeItem := anyMap(raw)
		names = appendUniqueText(names, stringFromMap(typeItem, "name"))
	}
	for _, raw := range anySlice(rawMutation["predictionItems"]) {
		typeItem := anyMap(raw)
		names = appendUniqueText(names, stringFromMap(typeItem, "typeName"))
	}
	for _, key := range []string{"observedTypeNames", "inspectedMutationTypeNames", "observedMutationTypeNames"} {
		for _, name := range textListFromAny(firstExistingAny(rawMutation[key], item[key])) {
			names = appendUniqueText(names, name)
		}
	}
	for _, key := range []string{"activeMutantTypes", "mutantTypes", "startedMutantIds", "lastHarvestMutantTypes"} {
		for _, id := range numberListFromAny(firstExistingAny(rawMutation[key], item[key])) {
			names = appendUniqueText(names, mutationRuntimeTypeNames[id])
		}
	}
	for _, raw := range anySlice(rawMutation["stageMutants"]) {
		stageMutant := anyMap(raw)
		id := firstPositiveInt(
			intFromAny(raw),
			intFromMap(stageMutant, "typeId"),
			intFromMap(stageMutant, "mutantType"),
			intFromMap(stageMutant, "mutantId"),
			intFromMap(stageMutant, "id"),
			intFromMap(stageMutant, "type"),
		)
		names = appendUniqueText(names, mutationRuntimeTypeNames[id])
	}
	sort.SliceStable(names, func(i, j int) bool {
		return mutationDisplayRank(names[i]) < mutationDisplayRank(names[j])
	})
	return names
}

func mutationDisplayRank(name string) int {
	for index, value := range mutationDisplayOrder {
		if value == name {
			return index
		}
	}
	return len(mutationDisplayOrder)
}

func growPhaseIndexByName(growPhases string, phaseName string) int {
	target := normalizedLookupText(phaseName)
	if target == "" {
		return 0
	}
	parts := strings.Split(growPhases, ";")
	for index, part := range parts {
		name := strings.TrimSpace(strings.Split(part, ":")[0])
		if normalizedLookupText(name) == target {
			return index + 1
		}
	}
	return 0
}

func readStageEntries(dir string) []landStageImageEntry {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	entries := make([]landStageImageEntry, 0, len(files))
	for _, file := range files {
		if file.IsDir() || !isImageFilename(file.Name()) {
			continue
		}
		entry := landStageImageEntry{Path: filepath.Join(dir, file.Name())}
		base := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		parts := strings.Split(base, "_")
		if len(parts) >= 3 {
			entry.Index = parseLeadingInt(parts[1])
			entry.Label = strings.Join(parts[2:], "_")
		}
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Index < entries[j].Index
	})
	return entries
}

func readStageEntriesForCrop(dir string, cropName string) []landStageImageEntry {
	entries := readStageEntries(dir)
	prefix := strings.TrimSpace(cropName) + "_"
	if prefix == "_" {
		return entries
	}
	filtered := make([]landStageImageEntry, 0, len(entries))
	for _, entry := range entries {
		base := strings.TrimSuffix(filepath.Base(entry.Path), filepath.Ext(entry.Path))
		if strings.HasPrefix(base, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func primaryStageImagePath(entries []landStageImageEntry) string {
	if len(entries) == 0 {
		return ""
	}
	for _, entry := range entries {
		label := normalizedLookupText(entry.Label)
		if entry.Index == 0 || label == normalizedLookupText("作物图") || label == normalizedLookupText("主图") {
			return entry.Path
		}
	}
	for _, entry := range entries {
		if entry.Index == 1 {
			return entry.Path
		}
	}
	return entries[0].Path
}

func imageFirstExistingPath(paths []string) string {
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

func firstImageInDir(dir string) string {
	files, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, file := range files {
		if !file.IsDir() && isImageFilename(file.Name()) {
			return filepath.Join(dir, file.Name())
		}
	}
	return ""
}

func isImageFilename(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		return true
	default:
		return false
	}
}

func collectUniqueText(values ...string) []string {
	result := []string{}
	for _, value := range values {
		result = appendUniqueText(result, value)
	}
	return result
}

func appendUniqueText(values []string, value string) []string {
	text := strings.TrimSpace(value)
	if text == "" {
		return values
	}
	normalized := normalizedLookupText(text)
	for _, existing := range values {
		if normalizedLookupText(existing) == normalized {
			return values
		}
	}
	return append(values, text)
}

func normalizedLookupText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func stripGoldCropNameGo(name string) string {
	text := strings.TrimSpace(name)
	text = strings.TrimPrefix(text, "黄金·")
	text = strings.TrimPrefix(text, "黄金")
	return strings.TrimSpace(text)
}

func textListFromAny(value any) []string {
	result := []string{}
	if value == nil {
		return result
	}
	for _, item := range anySliceOrSingle(value) {
		text := strings.TrimSpace(fmt.Sprint(item))
		if text != "" {
			result = appendUniqueText(result, text)
		}
	}
	return result
}

func numberListFromAny(value any) []int {
	result := []int{}
	if value == nil {
		return result
	}
	for _, item := range anySliceOrSingle(value) {
		if number := intFromAny(item); number > 0 {
			seen := false
			for _, existing := range result {
				if existing == number {
					seen = true
					break
				}
			}
			if !seen {
				result = append(result, number)
			}
		}
	}
	return result
}

func anySliceOrSingle(value any) []any {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []any:
		return typed
	case []map[string]any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	}
	return []any{value}
}

func firstExistingAny(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func parseLeadingInt(value string) int {
	var parsed int
	_, _ = fmt.Sscanf(strings.TrimSpace(value), "%d", &parsed)
	return parsed
}

func BuildRuntimeAtlasPreview(runtimeResult map[string]any) AtlasPreviewPayload {
	resolver := newLandImageResolver(DefaultGameConfigRoot())
	cropMeta := loadCropAtlasMetaIndex(resolver.root)
	mutationMeta := loadMutationAtlasMetaIndex(resolver.root)
	cropItems := atlasItemsFromRuntime(anyMap(runtimeResult["crop"]), "crop", resolver, cropMeta, mutationMeta)
	mutationItems := atlasItemsFromRuntime(anyMap(runtimeResult["mutation"]), "mutation", resolver, cropMeta, mutationMeta)
	sections := []AtlasSection{
		{ID: "crop", Label: "作物图鉴", Items: cropItems, Total: len(cropItems), Summary: summarizeAtlasItems(cropItems)},
		{ID: "mutation", Label: "超变图鉴", Items: mutationItems, Total: len(mutationItems), Summary: summarizeAtlasItems(mutationItems)},
	}
	return AtlasPreviewPayload{
		Source:         "runtime",
		Status:         "runtime",
		Message:        "图鉴已从游戏运行时刷新。",
		RefreshEnabled: true,
		BuyEnabled:     true,
		Sections:       sections,
	}
}

func BuildAtlasLockedCropPurchasePlan(input AtlasLockedCropPurchaseInput) AtlasPurchasePlan {
	count := input.CountPerSeed
	if count <= 0 {
		count = 1
	}
	if count > 99 {
		count = 99
	}
	level := input.EffectiveLevel
	if level < 0 {
		level = 0
	}
	resolver := newLandImageResolver(DefaultGameConfigRoot())
	shopBySeedID := map[int]map[string]any{}
	for _, raw := range input.ShopList {
		item := anyMap(raw)
		seedID := firstPositiveInt(intFromMap(item, "itemId"), intFromMap(item, "seedId"))
		goodsID := firstPositiveInt(intFromMap(item, "goodsId"), intFromMap(item, "id"), intFromMap(item, "gid"))
		if seedID > 0 && goodsID > 0 {
			if _, exists := shopBySeedID[seedID]; !exists {
				shopBySeedID[seedID] = item
			}
		}
	}

	plan := AtlasPurchasePlan{
		Purchases: []AtlasSeedPurchase{},
		Skipped:   []AtlasPurchaseSkipped{},
		Summary: AtlasPurchasePlanSummary{
			Requested:      len(input.AtlasItems),
			SkipReasons:    map[string]int{},
			CountPerSeed:   count,
			EffectiveLevel: level,
		},
	}
	skip := func(item AtlasItem, reason string, requiredLevel int) {
		plan.Skipped = append(plan.Skipped, AtlasPurchaseSkipped{
			Name:           item.Name,
			SeedID:         item.SeedID,
			FruitID:        item.FruitID,
			Sort:           item.Sort,
			Reason:         reason,
			RequiredLevel:  requiredLevel,
			EffectiveLevel: level,
		})
		plan.Summary.SkipReasons[reason]++
	}

	for _, item := range input.AtlasItems {
		atlasType := strings.TrimSpace(item.AtlasType)
		if atlasType == "" {
			atlasType = "crop"
		}
		if atlasType != "crop" {
			skip(item, "mutation_ignored", 0)
			continue
		}
		if !item.Locked {
			skip(item, "already_unlocked", 0)
			continue
		}
		plan.Summary.LockedCropCount++
		seedID := item.SeedID
		plant := resolver.plantByRuntimeIdentity(item.FruitID, seedID, item.Name)
		if seedID <= 0 && plant != nil {
			seedID = plant.SeedID
		}
		if seedID <= 0 {
			skip(item, "missing_seed", 0)
			continue
		}
		requiredLevel := item.Level
		if plant != nil && plant.LandLevelNeed > requiredLevel {
			requiredLevel = plant.LandLevelNeed
		}
		if requiredLevel > 0 && (level <= 0 || requiredLevel > level) {
			skip(item, "level_locked", requiredLevel)
			continue
		}
		shopItem, ok := shopBySeedID[seedID]
		if !ok {
			item.SeedID = seedID
			skip(item, "not_in_shop", requiredLevel)
			continue
		}
		shopRequiredLevel := firstPositiveInt(intFromMap(shopItem, "unlockLevel"), intFromMap(shopItem, "level"))
		if shopRequiredLevel > requiredLevel {
			requiredLevel = shopRequiredLevel
		}
		if requiredLevel > 0 && (level <= 0 || requiredLevel > level) {
			skip(item, "level_locked", requiredLevel)
			continue
		}
		name := firstNonEmptyString(item.Name)
		if plant != nil {
			name = firstNonEmptyString(name, plant.Name)
		}
		if name == "" {
			name = fmt.Sprintf("seed=%d", seedID)
		}
		plan.Purchases = append(plan.Purchases, AtlasSeedPurchase{
			SeedID:        seedID,
			SeedName:      name,
			GoodsID:       firstPositiveInt(intFromMap(shopItem, "goodsId"), intFromMap(shopItem, "id"), intFromMap(shopItem, "gid")),
			Price:         firstPositiveInt(intFromMap(shopItem, "price"), intFromMap(shopItem, "cost"), intFromMap(shopItem, "buyPrice")),
			Count:         count,
			RequiredLevel: requiredLevel,
			Sort:          item.Sort,
		})
	}
	sort.SliceStable(plan.Purchases, func(i, j int) bool {
		if plan.Purchases[i].Sort != plan.Purchases[j].Sort {
			return plan.Purchases[i].Sort < plan.Purchases[j].Sort
		}
		return plan.Purchases[i].SeedID < plan.Purchases[j].SeedID
	})
	plan.Summary.Purchasable = len(plan.Purchases)
	plan.Summary.Skipped = len(plan.Skipped)
	return plan
}

func AtlasItemsFromAny(value any) []AtlasItem {
	rows := anySlice(value)
	items := make([]AtlasItem, 0, len(rows))
	for index, raw := range rows {
		row := anyMap(raw)
		if len(row) == 0 {
			continue
		}
		locked := boolFromMap(row, "locked") || boolFromMap(row, "isLock")
		unlocked := boolFromMap(row, "unlocked") || boolFromMap(row, "isUnlock") || boolFromMap(row, "isUnlocked")
		if !locked && !unlocked {
			unlocked = true
		}
		items = append(items, AtlasItem{
			ID:          firstPositiveInt(intFromMap(row, "id"), intFromMap(row, "fruitId"), index+1),
			Name:        firstNonEmptyString(stringFromMap(row, "name"), stringFromMap(row, "fruitName")),
			SeedID:      intFromMap(row, "seedId"),
			FruitID:     intFromMap(row, "fruitId"),
			FruitType:   intFromMap(row, "fruitType"),
			FruitLayer:  intFromMap(row, "fruitLayer"),
			FruitRarity: intFromMap(row, "fruitRarity"),
			Progress:    intFromMap(row, "progress"),
			Level:       intFromMap(row, "level"),
			Seasons:     firstPositiveInt(intFromMap(row, "seasons"), 1),
			GrowTime:    intFromMap(row, "growTime"),
			Locked:      locked,
			Unlocked:    unlocked,
			CanUpgrade:  optionalBoolPointer(row, "canUpgrade"),
			IsNew:       optionalBoolPointer(row, "isNew"),
			Sort:        intFromMap(row, "sort"),
			AtlasType:   firstNonEmptyString(stringFromMap(row, "atlasType"), "crop"),
			ImageURL:    stringFromMap(row, "imageUrl"),
		})
	}
	return items
}

func DefaultGameConfigRoot() string {
	gameConfigRootOverride.RLock()
	override := gameConfigRootOverride.root
	gameConfigRootOverride.RUnlock()
	if override != "" {
		return override
	}
	relative := filepath.Join("resources", "gameConfig")
	candidates := []string{}
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		candidates = append(candidates,
			filepath.Join(cwd, relative),
			filepath.Join(cwd, "..", relative),
			filepath.Join(cwd, "..", "..", relative),
			filepath.Join(cwd, "..", "..", "..", relative),
		)
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, relative),
			filepath.Join(exeDir, "..", relative),
			filepath.Join(exeDir, "..", "..", relative),
			filepath.Join(exeDir, "..", "..", "..", relative),
		)
	}
	candidates = append(candidates,
		relative,
		filepath.Join("..", relative),
		filepath.Join("..", "..", relative),
		filepath.Join("..", "..", "..", relative),
	)

	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, "Plant.json")); err == nil {
			return candidate
		}
	}
	return relative
}

func loadGameConfig(root string) ([]plantConfigItem, map[int]itemInfoConfigItem, error) {
	var plants []plantConfigItem
	if err := readGameConfigJSON(root, "Plant.json", &plants); err != nil {
		return nil, nil, err
	}

	var itemInfo []itemInfoConfigItem
	if err := readGameConfigJSON(root, "ItemInfo.json", &itemInfo); err != nil {
		return nil, nil, err
	}

	itemMap := make(map[int]itemInfoConfigItem, len(itemInfo))
	for _, item := range itemInfo {
		if item.ID > 0 {
			itemMap[item.ID] = item
		}
	}
	return plants, itemMap, nil
}

func LoadSmartFertilizerCropIndex(root string) (SmartFertilizerCropIndex, error) {
	var plants []plantConfigItem
	if err := readGameConfigJSON(root, "Plant.json", &plants); err != nil {
		return SmartFertilizerCropIndex{}, err
	}

	index := SmartFertilizerCropIndex{
		byPlantID: make(map[int]SmartFertilizerCropClass, len(plants)),
		bySeedID:  make(map[int]SmartFertilizerCropClass, len(plants)),
	}
	for _, plant := range plants {
		class := smartFertilizerCropClassFromGrowPhases(plant.GrowPhases)
		if plant.ID > 0 {
			index.byPlantID[plant.ID] = class
		}
		if plant.SeedID > 0 {
			index.bySeedID[plant.SeedID] = class
		}
	}
	return index, nil
}

func smartFertilizerCropClassFromGrowPhases(growPhases string) SmartFertilizerCropClass {
	maxDuration := 0
	largeLeafDuration := -1
	for _, rawPhase := range strings.Split(growPhases, ";") {
		parts := strings.SplitN(strings.TrimSpace(rawPhase), ":", 2)
		if len(parts) != 2 {
			continue
		}
		duration, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || duration <= 0 {
			continue
		}
		if duration > maxDuration {
			maxDuration = duration
		}
		if strings.TrimSpace(parts[0]) == "大叶子" {
			largeLeafDuration = duration
		}
	}
	if maxDuration > 0 && largeLeafDuration == maxDuration {
		return SmartFertilizerCropSmart
	}
	return SmartFertilizerCropFallback
}

func LoadItemInfoMap(root string) (map[int]itemInfoConfigItem, error) {
	_, itemMap, err := loadGameConfig(root)
	return itemMap, err
}

func loadCanonicalLevelCropSeedOrder(root string) (map[int]int, error) {
	var mapping levelCropMapping
	if err := readGameConfigJSON(root, filepath.Join("plant_images", "stages", "_mappings", "crop_level_mapping.json"), &mapping); err != nil {
		return nil, err
	}
	seedOrder := make(map[int]int, len(mapping.Items))
	for index, item := range mapping.Items {
		if item.SeedID > 0 {
			seedOrder[item.SeedID] = index
		}
	}
	return seedOrder, nil
}

func readGameConfigJSON(root string, name string, target any) error {
	if root == "" {
		root = filepath.Join("resources", "gameConfig")
	}
	raw, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}
	return nil
}

func cropAnalyticsItem(plant plantConfigItem, itemMap map[int]itemInfoConfigItem) (CropAnalyticsItem, bool) {
	seasons := plant.Seasons
	if seasons <= 0 {
		seasons = 1
	}
	growTime := parseGrowTime(plant.GrowPhases, seasons)
	if growTime <= 0 {
		return CropAnalyticsItem{}, false
	}

	harvestExp := plant.Exp
	if seasons == 2 {
		harvestExp *= 2
	}
	seedPrice := itemMap[plant.SeedID].Price
	fruitPrice := itemMap[plant.Fruit.ID].Price
	income := fruitPrice * plant.Fruit.Count
	if seasons == 2 {
		income *= 2
	}
	netProfit := income - seedPrice
	reduceSecApplied := parseNormalFertilizerReduceSec(plant.GrowPhases, seasons)
	fertilizedTime := growTime - reduceSecApplied
	if fertilizedTime < 1 {
		fertilizedTime = 1
	}
	plantSize := plant.Size
	if plantSize <= 0 {
		plantSize = 1
	}

	var level *int
	if plant.LandLevelNeed > 0 {
		value := plant.LandLevelNeed
		level = &value
	}

	name := strings.TrimSpace(plant.Name)
	if name == "" {
		name = "未知作物"
	}

	return CropAnalyticsItem{
		ID:                            plant.ID,
		SeedID:                        plant.SeedID,
		Name:                          name,
		Seasons:                       seasons,
		Level:                         level,
		GrowTime:                      growTime,
		GrowTimeText:                  formatGrowTime(growTime),
		ReduceSecApplied:              reduceSecApplied,
		HarvestExp:                    harvestExp,
		ExpPerHour:                    round2(hourly(float64(harvestExp), growTime)),
		NormalFertilizerExpPerHour:    round2(hourly(float64(harvestExp), fertilizedTime)),
		Income:                        income,
		NetProfit:                     netProfit,
		ProfitPerHour:                 round2(hourly(float64(netProfit), growTime)),
		NormalFertilizerProfitPerHour: round2(hourly(float64(netProfit), fertilizedTime)),
		FruitID:                       plant.Fruit.ID,
		FruitCount:                    plant.Fruit.Count,
		FruitPrice:                    fruitPrice,
		SeedPrice:                     seedPrice,
		PlantSize:                     plantSize,
		ShopEligible:                  true,
	}, true
}

func isShopEligiblePlant(plant plantConfigItem, itemMap map[int]itemInfoConfigItem) bool {
	if plant.SeedID <= 0 || plant.LandLevelNeed <= 0 || plant.LandLevelNeed >= 200 {
		return false
	}
	if plant.Size > 1 {
		return false
	}
	seedItem, ok := itemMap[plant.SeedID]
	if !ok {
		return false
	}
	return seedItem.Type == 5 &&
		strings.TrimSpace(seedItem.InteractionType) == "plant" &&
		seedItem.PriceID == 0 &&
		seedItem.Price > 0 &&
		strings.TrimSpace(seedItem.Jumps) == levelShopJumpTarget
}

func plantStrategyModes() []PlantStrategyMode {
	return []PlantStrategyMode{
		{Value: "highest_level", Label: "最高等级作物"},
		{Value: "max_exp", Label: "经验/小时最高"},
		{Value: "max_fert_exp", Label: "普肥经验/小时最高"},
		{Value: "max_profit", Label: "净利润/小时最高"},
		{Value: "max_fert_profit", Label: "普肥净利润/小时最高"},
	}
}

func buildRecommendationCards(items []CropAnalyticsItem, modes []PlantStrategyMode, seedList []any, shopList []any) []PlantRecommendationCard {
	result := make([]PlantRecommendationCard, 0, len(modes))
	backpack := seedIDSet(seedList)
	shop := seedIDSet(shopList)
	for _, mode := range modes {
		sorted := append([]CropAnalyticsItem{}, items...)
		sortCropAnalyticsItems(sorted, recommendationSortKey(mode.Value))
		theoretical := firstCropPtr(sorted)
		current, source := firstAvailableCrop(sorted, backpack, shop)
		if current == nil {
			current = theoretical
			if current != nil {
				source = "static"
			}
		}
		result = append(result, PlantRecommendationCard{
			Value:                  mode.Value,
			Label:                  mode.Label,
			Recommended:            cloneCropPtr(current),
			CurrentRecommended:     cloneCropPtr(current),
			CurrentSource:          source,
			TheoreticalRecommended: cloneCropPtr(theoretical),
		})
	}
	return result
}

func recommendationSortKey(mode string) string {
	switch mode {
	case "highest_level":
		return "level"
	case "max_fert_exp":
		return "fert_exp"
	case "max_profit":
		return "profit"
	case "max_fert_profit":
		return "fert_profit"
	default:
		return "exp"
	}
}

func sortCropAnalyticsItems(items []CropAnalyticsItem, sortKey string) {
	sort.SliceStable(items, func(i, j int) bool {
		switch sortKey {
		case "level":
			if levelValue(items[i]) != levelValue(items[j]) {
				return levelValue(items[i]) > levelValue(items[j])
			}
		case "grow_time":
			if items[i].GrowTime != items[j].GrowTime {
				return items[i].GrowTime < items[j].GrowTime
			}
		case "fert_exp":
			if items[i].NormalFertilizerExpPerHour != items[j].NormalFertilizerExpPerHour {
				return items[i].NormalFertilizerExpPerHour > items[j].NormalFertilizerExpPerHour
			}
		case "profit":
			if items[i].ProfitPerHour != items[j].ProfitPerHour {
				return items[i].ProfitPerHour > items[j].ProfitPerHour
			}
		case "fert_profit":
			if items[i].NormalFertilizerProfitPerHour != items[j].NormalFertilizerProfitPerHour {
				return items[i].NormalFertilizerProfitPerHour > items[j].NormalFertilizerProfitPerHour
			}
		default:
			if items[i].ExpPerHour != items[j].ExpPerHour {
				return items[i].ExpPerHour > items[j].ExpPerHour
			}
		}
		return items[i].canonicalOrder < items[j].canonicalOrder
	})
}

func levelValue(item CropAnalyticsItem) int {
	if item.Level == nil {
		return 0
	}
	return *item.Level
}

func firstCropPtr(items []CropAnalyticsItem) *CropAnalyticsItem {
	if len(items) == 0 {
		return nil
	}
	return &items[0]
}

func cloneCropPtr(item *CropAnalyticsItem) *CropAnalyticsItem {
	if item == nil {
		return nil
	}
	clone := *item
	return &clone
}

func firstAvailableCrop(items []CropAnalyticsItem, backpack map[int]bool, shop map[int]bool) (*CropAnalyticsItem, string) {
	for i := range items {
		seedID := items[i].SeedID
		if backpack[seedID] {
			return &items[i], "backpack"
		}
		if shop[seedID] {
			return &items[i], "shop"
		}
	}
	return nil, ""
}

func seedIDSet(items []any) map[int]bool {
	result := map[int]bool{}
	for _, raw := range items {
		item := anyMap(raw)
		seedID := firstPositiveInt(
			intFromMap(item, "seedId"),
			intFromMap(item, "itemId"),
			intFromMap(item, "id"),
		)
		if seedID > 0 {
			result[seedID] = true
		}
	}
	return result
}

func parseGrowTime(growPhases string, seasons int) int {
	durations := parseGrowPhaseDurations(growPhases)
	if len(durations) == 0 {
		return 0
	}
	total := 0
	for _, duration := range durations {
		total += duration
	}
	if seasons != 2 {
		return total
	}
	lastTwo := lastPositiveDurations(durations, 2)
	for _, duration := range lastTwo {
		total += duration
	}
	return total
}

func parseNormalFertilizerReduceSec(growPhases string, seasons int) int {
	durations := parseGrowPhaseDurations(growPhases)
	maxDuration := 0
	for _, duration := range durations {
		if duration > maxDuration {
			maxDuration = duration
		}
	}
	if maxDuration == 0 {
		return 0
	}
	if seasons == 2 {
		return maxDuration * 2
	}
	return maxDuration
}

func parseGrowPhaseDurations(growPhases string) []int {
	parts := strings.Split(growPhases, ";")
	durations := make([]int, 0, len(parts))
	for _, part := range parts {
		match := growPhaseDurationPattern.FindStringSubmatch(part)
		if len(match) != 2 {
			continue
		}
		var duration int
		_, _ = fmt.Sscanf(match[1], "%d", &duration)
		durations = append(durations, duration)
	}
	return durations
}

func lastPositiveDurations(values []int, count int) []int {
	result := make([]int, 0, count)
	for i := len(values) - 1; i >= 0 && len(result) < count; i-- {
		if values[i] > 0 {
			result = append(result, values[i])
		}
	}
	return result
}

func formatGrowTime(seconds int) string {
	if seconds <= 0 {
		return "0秒"
	}
	if seconds < 60 {
		return fmt.Sprintf("%d秒", seconds)
	}
	if seconds < 3600 {
		mins := seconds / 60
		secs := seconds % 60
		if secs > 0 {
			return fmt.Sprintf("%d分%d秒", mins, secs)
		}
		return fmt.Sprintf("%d分", mins)
	}
	hours := seconds / 3600
	mins := (seconds % 3600) / 60
	if mins > 0 {
		return fmt.Sprintf("%d时%d分", hours, mins)
	}
	return fmt.Sprintf("%d时", hours)
}

func hourly(value float64, seconds int) float64 {
	if seconds <= 0 {
		return 0
	}
	return value / float64(seconds) * 3600
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func atlasItemsFromRuntime(section map[string]any, atlasType string, resolver landImageResolver, cropMeta cropAtlasMetaIndex, mutationMeta mutationAtlasMetaIndex) []AtlasItem {
	rows := anySlice(section["items"])
	items := make([]AtlasItem, 0, len(rows))
	for index, raw := range rows {
		row := anyMap(raw)
		fruitID := intFromMap(row, "fruitId")
		seedID := intFromMap(row, "seedId")
		name := firstNonEmptyString(stringFromMap(row, "name"), stringFromMap(row, "fruitName"))
		crop := cropAtlasMeta{}
		if atlasType == "crop" {
			crop = cropMeta.byRuntimeIdentity(fruitID, seedID, name)
			name = firstNonEmptyString(name, crop.Name)
			if seedID <= 0 {
				seedID = crop.SeedID
			}
			if fruitID <= 0 {
				fruitID = crop.FruitID
			}
		}
		meta := mutationAtlasMeta{}
		if atlasType == "mutation" {
			meta = mutationMeta.byRuntimeID(fruitID)
			if meta.Name == "" && name != "" {
				meta = mutationMeta.byPlantName(name)
			}
			name = firstNonEmptyString(name, meta.Name)
		}
		var plant *plantConfigItem
		if atlasType == "crop" {
			plant = resolver.plantByCropIdentity(fruitID, seedID, name)
		} else {
			plant = resolver.plantByRuntimeIdentity(fruitID, seedID, name)
		}
		if plant != nil {
			name = firstNonEmptyString(name, plant.Name)
			if seedID <= 0 {
				seedID = plant.SeedID
			}
			if fruitID <= 0 {
				fruitID = plant.Fruit.ID
			}
		}
		locked := boolFromMap(row, "locked") || boolFromMap(row, "isLock")
		unlocked := boolFromMap(row, "unlocked") || boolFromMap(row, "isUnlock") || boolFromMap(row, "isUnlocked")
		if !unlocked && !locked {
			if _, ok := optionalBoolFromMap(row, "unlocked"); ok {
				locked = !unlocked
			} else if _, ok := optionalBoolFromMap(row, "isUnlock"); ok {
				locked = !unlocked
			} else if _, ok := optionalBoolFromMap(row, "isUnlocked"); ok {
				locked = !unlocked
			} else {
				unlocked = true
			}
		}
		level := intFromMap(row, "level")
		seasons := firstPositiveInt(intFromMap(row, "seasons"), 1)
		growTime := intFromMap(row, "growTime")
		if atlasType == "crop" {
			level = firstPositiveInt(level, crop.Level)
			seasons = firstPositiveInt(intFromMap(row, "seasons"), crop.Seasons, 1)
			growTime = firstPositiveInt(growTime, parseGrowTime(crop.GrowPhases, seasons))
		}
		if plant != nil {
			level = firstPositiveInt(level, crop.Level, plant.LandLevelNeed)
			seasons = firstPositiveInt(intFromMap(row, "seasons"), plant.Seasons, 1)
			growTime = firstPositiveInt(growTime, parseGrowTime(plant.GrowPhases, seasons))
		}
		progress := intFromMap(row, "progress")
		imageURL := ""
		if atlasType == "mutation" {
			progress = firstPositiveInt(progress, meta.AtlasPoint)
			if meta.ImagePath != "" {
				imageURL = gameConfigLocalImageURL(resolver.root, meta.ImagePath)
			}
			if imageURL == "" {
				imageURL = resolver.resolveMutationPlantImage(name, 0)
			}
		} else {
			imageURL = resolver.resolvePlantMainImage(seedID, name)
		}
		canUpgrade := optionalBoolPointer(row, "canUpgrade")
		isNew := optionalBoolPointer(row, "isNew")
		explicitSort := intFromMap(row, "sort")
		sortValue := firstPositiveInt(explicitSort)
		if atlasType == "crop" && sortValue <= 0 {
			sortValue = firstPositiveInt(crop.AtlasOrder, level, index+1)
		}
		if atlasType == "mutation" {
			sortValue = firstPositiveInt(meta.AtlasOrder, sortValue, index+1)
		}
		items = append(items, AtlasItem{
			ID:          firstPositiveInt(intFromMap(row, "id"), fruitID, index+1),
			Name:        name,
			SeedID:      seedID,
			FruitID:     fruitID,
			GroupName:   meta.GroupName,
			FruitType:   intFromMap(row, "fruitType"),
			FruitLayer:  intFromMap(row, "fruitLayer"),
			FruitRarity: intFromMap(row, "fruitRarity"),
			Progress:    progress,
			Level:       level,
			Seasons:     seasons,
			GrowTime:    growTime,
			Locked:      locked,
			Unlocked:    unlocked,
			CanUpgrade:  canUpgrade,
			IsNew:       isNew,
			Sort:        sortValue,
			AtlasType:   atlasType,
			ImageURL:    imageURL,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if atlasType == "mutation" {
			leftGroup := mutationAtlasGroupRank(items[i])
			rightGroup := mutationAtlasGroupRank(items[j])
			if leftGroup != rightGroup {
				return leftGroup < rightGroup
			}
			leftPoint := items[i].Progress
			rightPoint := items[j].Progress
			if leftPoint <= 0 {
				leftPoint = int(^uint(0) >> 1)
			}
			if rightPoint <= 0 {
				rightPoint = int(^uint(0) >> 1)
			}
			if leftPoint != rightPoint {
				return leftPoint < rightPoint
			}
		}
		if items[i].Sort != items[j].Sort {
			return items[i].Sort < items[j].Sort
		}
		if items[i].Level != items[j].Level {
			return items[i].Level < items[j].Level
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func mutationAtlasGroupRank(item AtlasItem) int {
	groupName := strings.TrimSpace(item.GroupName)
	if groupName == "黄金果实" || strings.HasPrefix(item.Name, "黄金·") {
		return 0
	}
	if groupName == "装扮果实" {
		return 1
	}
	if groupName == "活动果实" {
		return 2
	}
	return 3
}

func summarizeAtlasItems(items []AtlasItem) AtlasSectionSummary {
	summary := AtlasSectionSummary{Total: len(items)}
	for _, item := range items {
		if item.Unlocked {
			summary.Unlocked++
		}
		if item.Locked {
			summary.Locked++
		}
	}
	return summary
}

func summarizeWarehousePayload(items []WarehouseItem) WarehouseSummary {
	var summary WarehouseSummary
	categoryMap := map[string]*WarehouseCategorySummary{}
	for _, category := range warehouseCategoryOrder {
		item := category
		categoryMap[item.Key] = &item
	}
	summary.TotalDistinct = len(items)
	for _, item := range items {
		summary.TotalCount += item.Count
		key := strings.TrimSpace(item.Category)
		if key == "" {
			key = "tool"
		}
		if _, ok := categoryMap[key]; !ok {
			key = "tool"
		}
		categoryMap[key].Distinct++
		categoryMap[key].Count += item.Count
		if item.CanSell {
			summary.SellableDistinct++
			summary.SellableCount += item.Count
			summary.EstimatedAllSellPrice += item.EstimatedSellPrice
		}
	}
	for _, category := range warehouseCategoryOrder {
		summary.CategoryList = append(summary.CategoryList, *categoryMap[category.Key])
	}
	return summary
}

func resolveWarehouseItemImageURL(resolver landImageResolver, itemID int, name string, category string) string {
	switch strings.TrimSpace(strings.ToLower(category)) {
	case "seed":
		if plant, ok := resolver.bySeedID[itemID]; ok {
			return resolver.resolvePlantMainImage(plant.SeedID, firstNonEmptyString(warehousePlantName(name), plant.Name))
		}
	case "fruit":
		if plant, ok := resolver.byFruitID[itemID]; ok {
			return resolver.resolvePlantMainImage(plant.SeedID, firstNonEmptyString(warehousePlantName(name), plant.Name))
		}
	case "mutation":
		if imageURL := resolver.resolveMutationPlantImage(name, 0); imageURL != "" {
			return imageURL
		}
		if imageURL := resolver.resolveMappedWarehouseItemImage(itemID, name); imageURL != "" {
			return imageURL
		}
	case "tool":
		if imageURL := resolver.resolveMappedWarehouseItemImage(itemID, name); imageURL != "" {
			return imageURL
		}
	}
	return gameConfigLocalImageURL(resolver.root, imageFirstExistingPath([]string{
		filepath.Join(resolver.root, "plant_images", "default", strconv.Itoa(itemID)+".png"),
		filepath.Join(resolver.root, "plant_images", "default", "400.png"),
		filepath.Join(resolver.root, "plant_images", "default", "400.jpg"),
		filepath.Join(resolver.root, "plant_images", "default", "400.jpeg"),
	}))
}

func (r landImageResolver) resolvePlantMainImage(seedID int, plantName string) string {
	path := r.resolvePlantMainImagePath(seedID, plantName)
	return gameConfigLocalImageURL(r.root, path)
}

func (r landImageResolver) resolvePlantMainImagePath(seedID int, plantName string) string {
	plant := r.plantByRuntimeIdentity(0, seedID, plantName)
	names := collectUniqueText(warehousePlantName(plantName))
	if plant != nil {
		names = appendUniqueText(names, plant.Name)
	}
	for _, name := range names {
		if path := primaryStageImagePath(readStageEntries(filepath.Join(r.root, "plant_images", "stages", "作物", name))); path != "" {
			return path
		}
		for _, goldName := range collectUniqueText(name, "黄金·"+name) {
			if path := primaryStageImagePath(readStageEntries(filepath.Join(r.root, "plant_images", "stages", "黄金果实", goldName))); path != "" {
				return path
			}
		}
	}
	if meta := r.activityCrops.find(plantIDOf(plant), seedID, fruitIDOf(plant), plantName); meta != nil {
		if path := r.resolveActivityCropMainImagePath(*meta, append(names, plantName)...); path != "" {
			return path
		}
	}
	return imageFirstExistingPath([]string{
		filepath.Join(r.root, "plant_images", "default", "400.png"),
		filepath.Join(r.root, "plant_images", "default", "400.jpg"),
		filepath.Join(r.root, "plant_images", "default", "400.jpeg"),
	})
}

func (r landImageResolver) resolveMappedWarehouseItemImage(itemID int, name string) string {
	if meta := r.activityCrops.find(0, 0, itemID, name); meta != nil {
		if path := r.resolveActivityCropMainImagePath(*meta, name); path != "" {
			return gameConfigLocalImageURL(r.root, path)
		}
	}
	stagesRoot := filepath.Join(r.root, "plant_images", "stages")
	mappingRoots := []string{
		filepath.Join(stagesRoot, "_mappings", "requested_category_image_id_mapping.csv"),
		filepath.Join(stagesRoot, "_mappings", "超变果实_mapping.csv"),
		filepath.Join(stagesRoot, "_mappings", "活动货币_mapping.csv"),
		filepath.Join(stagesRoot, "超变果实", "超变果实_mapping.csv"),
		filepath.Join(stagesRoot, "活动货币", "活动货币_mapping.csv"),
	}
	for _, mappingPath := range mappingRoots {
		if imagePath := mappedWarehouseItemImagePath(stagesRoot, mappingPath, itemID, name); imagePath != "" {
			return gameConfigLocalImageURL(r.root, imagePath)
		}
	}
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return ""
	}
	for _, category := range []string{"活动货币", "化肥道具", "超变果实", "活动果实"} {
		if path := firstImageInDir(filepath.Join(stagesRoot, category, cleanName)); path != "" {
			return gameConfigLocalImageURL(r.root, path)
		}
	}
	return ""
}

func isLocalGameImageURL(value string) bool {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, localImageAssetPrefix) {
		return false
	}
	id := strings.TrimPrefix(value, localImageAssetPrefix)
	return id != "" && !strings.Contains(id, "/") && !strings.Contains(id, "\\")
}

func mappedWarehouseItemImagePath(stagesRoot string, mappingPath string, itemID int, name string) string {
	file, err := os.Open(mappingPath)
	if err != nil {
		return ""
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 {
		return ""
	}
	header := make(map[string]int, len(rows[0]))
	for index, key := range rows[0] {
		header[cleanCSVHeader(key)] = index
	}
	for _, row := range rows[1:] {
		rowItemID := csvInt(row, header, "item_id")
		if rowItemID == 0 {
			rowItemID = csvInt(row, header, "itemId")
		}
		rowName := csvString(row, header, "name")
		if itemID > 0 && rowItemID != itemID {
			continue
		}
		if itemID <= 0 && normalizedLookupText(rowName) != normalizedLookupText(name) {
			continue
		}
		if imagePath := firstExistingMappedImagePath(stagesRoot, filepath.Dir(mappingPath), csvString(row, header, "local_path")); imagePath != "" {
			return imagePath
		}
		if imagePath := firstExistingMappedImagePath(stagesRoot, filepath.Dir(mappingPath), csvString(row, header, "image_path")); imagePath != "" {
			return imagePath
		}
	}
	return ""
}

func csvString(row []string, header map[string]int, key string) string {
	index, ok := header[key]
	if !ok || index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func cleanCSVHeader(key string) string {
	return strings.TrimPrefix(strings.TrimSpace(key), "\ufeff")
}

func csvInt(row []string, header map[string]int, key string) int {
	value, _ := strconv.Atoi(csvString(row, header, key))
	return value
}

func firstExistingMappedImagePath(stagesRoot string, mappingDir string, localPath string) string {
	cleanPath := strings.TrimSpace(localPath)
	if cleanPath == "" || strings.HasPrefix(cleanPath, "http://") || strings.HasPrefix(cleanPath, "https://") {
		return ""
	}
	return imageFirstExistingPath([]string{
		filepath.Join(stagesRoot, cleanPath),
		filepath.Join(mappingDir, cleanPath),
		filepath.Join(filepath.Dir(mappingDir), cleanPath),
	})
}

func warehousePlantName(name string) string {
	clean := strings.TrimSpace(name)
	clean = strings.TrimSuffix(clean, "种子")
	return strings.TrimSpace(clean)
}

func warehouseSaleUnitPrice(item map[string]any) int {
	for _, key := range []string{"saleUnitPrice", "price"} {
		if value := intFromMap(item, key); value > 0 {
			return value
		}
	}
	for _, reward := range anySlice(item["saleRewards"]) {
		if amount := intFromMap(anyMap(reward), "amount"); amount > 0 {
			return amount
		}
	}
	for _, reward := range anySlice(item["sells"]) {
		if amount := intFromMap(anyMap(reward), "amount"); amount > 0 {
			return amount
		}
	}
	return 0
}

func warehouseCategory(itemID int, info itemInfoConfigItem) string {
	if warehouseMappedCategory(itemID) == "超变果实" {
		return "mutation"
	}
	switch info.Type {
	case 5:
		return "seed"
	case 6:
		return "fruit"
	case 17:
		return "mutation"
	case 1, 2, 7, 8, 9, 11, 15, 16, 19:
		return "tool"
	default:
		if itemID >= 20000 && itemID < 30000 {
			return "seed"
		}
		if itemID >= 40000 && itemID < 50000 {
			return "fruit"
		}
		return "tool"
	}
}

func warehouseMappedCategory(itemID int) string {
	if itemID <= 0 {
		return ""
	}
	mappingPath := filepath.Join(DefaultGameConfigRoot(), "plant_images", "stages", "_mappings", "requested_category_image_id_mapping.csv")
	file, err := os.Open(mappingPath)
	if err != nil {
		return ""
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 {
		return ""
	}
	header := make(map[string]int, len(rows[0]))
	for index, key := range rows[0] {
		header[cleanCSVHeader(key)] = index
	}
	for _, row := range rows[1:] {
		if csvInt(row, header, "item_id") == itemID || csvRowHasItemID(row, itemID) {
			return csvString(row, header, "category_name")
		}
	}
	return ""
}

func csvRowHasItemID(row []string, itemID int) bool {
	target := strconv.Itoa(itemID)
	for _, cell := range row {
		if strings.TrimSpace(cell) == target {
			return true
		}
	}
	return false
}

func warehouseCategoryLabel(category string) string {
	switch category {
	case "seed":
		return "种子"
	case "fruit":
		return "果实"
	case "mutation":
		return "超变果实"
	case "tool":
		return "道具"
	default:
		return "道具"
	}
}

func landTypeDisplayLabel(landType string) string {
	switch strings.TrimSpace(strings.ToLower(landType)) {
	case "purplegold", "purple_gold", "purple-gold":
		return "紫金土地"
	case "gold":
		return "金土地"
	case "black":
		return "黑土地"
	case "red":
		return "红土地"
	case "normal":
		return "普通土地"
	default:
		return ""
	}
}

func landStatusLabel(status string) string {
	switch status {
	case "empty":
		return "空地"
	case "mature":
		return "已成熟"
	case "growing":
		return "生长中"
	case "dead":
		return "枯萎"
	case "locked":
		return "未解锁"
	default:
		return "未知"
	}
}

func landMatureEtaText(status string, matureInSec *int) string {
	if matureInSec != nil {
		if *matureInSec <= 0 {
			return "已成熟"
		}
		return fmt.Sprintf("预计 %s 后成熟", formatLandMatureEta(*matureInSec))
	}
	switch status {
	case "empty":
		return "空地"
	case "locked":
		return "未解锁"
	default:
		return ""
	}
}

func formatLandMatureEta(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%02d:%02d", minutes, secs)
}

func anyMap(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	if typed, ok := value.(map[string]interface{}); ok {
		return typed
	}
	return map[string]any{}
}

func anySlice(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []map[string]any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	default:
		return nil
	}
}

func intFromMap(values map[string]any, key string) int {
	if values == nil {
		return 0
	}
	return intFromAny(values[key])
}

func optionalIntFromMap(values map[string]any, key string) (int, bool) {
	if values == nil {
		return 0, false
	}
	if _, ok := values[key]; !ok {
		return 0, false
	}
	return intFromAny(values[key]), true
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		var parsed int
		_, _ = fmt.Sscanf(strings.TrimSpace(typed), "%d", &parsed)
		return parsed
	default:
		return 0
	}
}

func boolFromMap(values map[string]any, key string) bool {
	if values == nil {
		return false
	}
	switch typed := values[key].(type) {
	case bool:
		return typed
	case string:
		text := strings.ToLower(strings.TrimSpace(typed))
		return text == "1" || text == "true" || text == "yes" || text == "on"
	case float64:
		return typed != 0
	case int:
		return typed != 0
	default:
		return false
	}
}

func optionalBoolFromMap(values map[string]any, key string) (bool, bool) {
	if values == nil {
		return false, false
	}
	value, ok := values[key]
	if !ok {
		return false, false
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		text := strings.ToLower(strings.TrimSpace(typed))
		if text == "1" || text == "true" || text == "yes" || text == "on" {
			return true, true
		}
		if text == "0" || text == "false" || text == "no" || text == "off" {
			return false, true
		}
	case float64:
		return typed != 0, true
	case int:
		return typed != 0, true
	}
	return false, false
}

func optionalBoolPointer(values map[string]any, key string) *bool {
	if value, ok := optionalBoolFromMap(values, key); ok {
		return &value
	}
	return nil
}

func stringFromMap(values map[string]any, key string) string {
	if values == nil || values[key] == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(values[key]))
}

func mapFromAny(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func sourceLabel(root string) string {
	clean := filepath.ToSlash(filepath.Clean(root))
	if strings.HasSuffix(clean, "resources/gameConfig") {
		return "resources/gameConfig"
	}
	return clean
}
