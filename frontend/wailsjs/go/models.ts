export namespace app {
	
	export class ConnectionInfo {
	    url: string;
	    expectedHostVersion: string;
	    tokenPreview: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.expectedHostVersion = source["expectedHostVersion"];
	        this.tokenPreview = source["tokenPreview"];
	    }
	}

}

export namespace automation {
	
	export class ActionResult {
	    ok: boolean;
	    status: string;
	    taskId?: string;
	    message: string;
	    actionCount?: number;
	    attemptedFriends?: number;
	    successfulFriends?: number;
	    failedFriends?: number;
	    skippedFriends?: number;
	
	    static createFrom(source: any = {}) {
	        return new ActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.status = source["status"];
	        this.taskId = source["taskId"];
	        this.message = source["message"];
	        this.actionCount = source["actionCount"];
	        this.attemptedFriends = source["attemptedFriends"];
	        this.successfulFriends = source["successfulFriends"];
	        this.failedFriends = source["failedFriends"];
	        this.skippedFriends = source["skippedFriends"];
	    }
	}
	export class FeatureGroup {
	    id: string;
	    label: string;
	    summary: string;
	    enabled: boolean;
	    settingKeys: string[];
	
	    static createFrom(source: any = {}) {
	        return new FeatureGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.summary = source["summary"];
	        this.enabled = source["enabled"];
	        this.settingKeys = source["settingKeys"];
	    }
	}
	export class SchedulerTask {
	    id: string;
	    label: string;
	    enabledConfigKey?: string;
	    intervalConfigKey?: string;
	    priority: number;
	    intervalSec: number;
	    enabled: boolean;
	    dailyDoneToday?: boolean;
	    nextRunAt?: string;
	    lastStartedAt?: string;
	    lastFinishedAt?: string;
	    lastSuccessAt?: string;
	    lastError?: string;
	    lastResultSummary?: string;
	
	    static createFrom(source: any = {}) {
	        return new SchedulerTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.enabledConfigKey = source["enabledConfigKey"];
	        this.intervalConfigKey = source["intervalConfigKey"];
	        this.priority = source["priority"];
	        this.intervalSec = source["intervalSec"];
	        this.enabled = source["enabled"];
	        this.dailyDoneToday = source["dailyDoneToday"];
	        this.nextRunAt = source["nextRunAt"];
	        this.lastStartedAt = source["lastStartedAt"];
	        this.lastFinishedAt = source["lastFinishedAt"];
	        this.lastSuccessAt = source["lastSuccessAt"];
	        this.lastError = source["lastError"];
	        this.lastResultSummary = source["lastResultSummary"];
	    }
	}
	export class SchedulerState {
	    enabled: boolean;
	    minGapMs: number;
	    runningTaskId?: string;
	    tasks: SchedulerTask[];
	
	    static createFrom(source: any = {}) {
	        return new SchedulerState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.minGapMs = source["minGapMs"];
	        this.runningTaskId = source["runningTaskId"];
	        this.tasks = this.convertValues(source["tasks"], SchedulerTask);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class State {
	    running: boolean;
	    runMode: string;
	    summary: Record<string, number>;
	    featureGroups: FeatureGroup[];
	    scheduler: SchedulerState;
	    config: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.runMode = source["runMode"];
	        this.summary = source["summary"];
	        this.featureGroups = this.convertValues(source["featureGroups"], FeatureGroup);
	        this.scheduler = this.convertValues(source["scheduler"], SchedulerState);
	        this.config = source["config"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace diagnostics {
	
	export class Result {
	    method: string;
	    ok: boolean;
	    durationMs: number;
	    result?: any;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.method = source["method"];
	        this.ok = source["ok"];
	        this.durationMs = source["durationMs"];
	        this.result = source["result"];
	        this.error = source["error"];
	    }
	}

}

export namespace eventbus {
	
	export class Event {
	    id: number;
	    // Go type: time
	    timestamp: any;
	    level: string;
	    source: string;
	    type: string;
	    message: string;
	    data?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.level = source["level"];
	        this.source = source["source"];
	        this.type = source["type"];
	        this.message = source["message"];
	        this.data = source["data"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace farm {
	
	export class StatsHistoryWindow {
	    todayKey?: string;
	    days: DayStats[];
	
	    static createFrom(source: any = {}) {
	        return new StatsHistoryWindow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.todayKey = source["todayKey"];
	        this.days = this.convertValues(source["days"], DayStats);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DayStats {
	    dateKey?: string;
	    updatedAt?: string;
	    runs: number;
	    collect: number;
	    water: number;
	    steal: number;
	    help: number;
	    mischiefGrass: number;
	    mischiefBug: number;
	    sell: number;
	    saleEstimate: number;
	    estimateReady: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DayStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dateKey = source["dateKey"];
	        this.updatedAt = source["updatedAt"];
	        this.runs = source["runs"];
	        this.collect = source["collect"];
	        this.water = source["water"];
	        this.steal = source["steal"];
	        this.help = source["help"];
	        this.mischiefGrass = source["mischiefGrass"];
	        this.mischiefBug = source["mischiefBug"];
	        this.sell = source["sell"];
	        this.saleEstimate = source["saleEstimate"];
	        this.estimateReady = source["estimateReady"];
	    }
	}
	export class LevelProgress {
	    level?: number;
	    current: number;
	    needed: number;
	    remaining: number;
	    percent: number;
	    nextLevel?: number;
	
	    static createFrom(source: any = {}) {
	        return new LevelProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.current = source["current"];
	        this.needed = source["needed"];
	        this.remaining = source["remaining"];
	        this.percent = source["percent"];
	        this.nextLevel = source["nextLevel"];
	    }
	}
	export class AccountProfile {
	    gid?: number;
	    name?: string;
	    nick?: string;
	    level?: number;
	    plantLevel?: number;
	    farmMaxLandLevel?: number;
	    exp?: number;
	    nextLevelExp?: number;
	    gold?: number;
	    money?: number;
	    bean?: number;
	    coupon?: number;
	    diamond?: number;
	    avatarUrl?: string;
	    levelProgress: LevelProgress;
	    todayStats: DayStats;
	    statsHistory: StatsHistoryWindow;
	
	    static createFrom(source: any = {}) {
	        return new AccountProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gid = source["gid"];
	        this.name = source["name"];
	        this.nick = source["nick"];
	        this.level = source["level"];
	        this.plantLevel = source["plantLevel"];
	        this.farmMaxLandLevel = source["farmMaxLandLevel"];
	        this.exp = source["exp"];
	        this.nextLevelExp = source["nextLevelExp"];
	        this.gold = source["gold"];
	        this.money = source["money"];
	        this.bean = source["bean"];
	        this.coupon = source["coupon"];
	        this.diamond = source["diamond"];
	        this.avatarUrl = source["avatarUrl"];
	        this.levelProgress = this.convertValues(source["levelProgress"], LevelProgress);
	        this.todayStats = this.convertValues(source["todayStats"], DayStats);
	        this.statsHistory = this.convertValues(source["statsHistory"], StatsHistoryWindow);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FertilizerSlotStatus {
	    available: boolean;
	    remainingSec: number;
	    remainingHours: number;
	    remainingText?: string;
	
	    static createFrom(source: any = {}) {
	        return new FertilizerSlotStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.remainingSec = source["remainingSec"];
	        this.remainingHours = source["remainingHours"];
	        this.remainingText = source["remainingText"];
	    }
	}
	export class FertilizerContainerStatus {
	    normal: FertilizerSlotStatus;
	    organic: FertilizerSlotStatus;
	
	    static createFrom(source: any = {}) {
	        return new FertilizerContainerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.normal = this.convertValues(source["normal"], FertilizerSlotStatus);
	        this.organic = this.convertValues(source["organic"], FertilizerSlotStatus);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AccountStatusPayload {
	    status: string;
	    message: string;
	    profile: AccountProfile;
	    fertilizer: FertilizerContainerStatus;
	    profileError?: string;
	    fertilizerError?: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountStatusPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.message = source["message"];
	        this.profile = this.convertValues(source["profile"], AccountProfile);
	        this.fertilizer = this.convertValues(source["fertilizer"], FertilizerContainerStatus);
	        this.profileError = source["profileError"];
	        this.fertilizerError = source["fertilizerError"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AnalyticsLevelInfo {
	    requestedMaxLevel: number;
	    effectiveMaxLevel: number;
	    levelSource: string;
	
	    static createFrom(source: any = {}) {
	        return new AnalyticsLevelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requestedMaxLevel = source["requestedMaxLevel"];
	        this.effectiveMaxLevel = source["effectiveMaxLevel"];
	        this.levelSource = source["levelSource"];
	    }
	}
	export class AtlasItem {
	    id: number;
	    name: string;
	    seedId: number;
	    fruitId: number;
	    groupName?: string;
	    fruitType?: number;
	    fruitLayer?: number;
	    fruitRarity?: number;
	    progress?: number;
	    level: number;
	    seasons: number;
	    growTime: number;
	    locked: boolean;
	    unlocked: boolean;
	    canUpgrade?: boolean;
	    isNew?: boolean;
	    sort?: number;
	    atlasType?: string;
	    imageUrl?: string;
	
	    static createFrom(source: any = {}) {
	        return new AtlasItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.seedId = source["seedId"];
	        this.fruitId = source["fruitId"];
	        this.groupName = source["groupName"];
	        this.fruitType = source["fruitType"];
	        this.fruitLayer = source["fruitLayer"];
	        this.fruitRarity = source["fruitRarity"];
	        this.progress = source["progress"];
	        this.level = source["level"];
	        this.seasons = source["seasons"];
	        this.growTime = source["growTime"];
	        this.locked = source["locked"];
	        this.unlocked = source["unlocked"];
	        this.canUpgrade = source["canUpgrade"];
	        this.isNew = source["isNew"];
	        this.sort = source["sort"];
	        this.atlasType = source["atlasType"];
	        this.imageUrl = source["imageUrl"];
	    }
	}
	export class AtlasSectionSummary {
	    total: number;
	    unlocked: number;
	    locked: number;
	
	    static createFrom(source: any = {}) {
	        return new AtlasSectionSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.unlocked = source["unlocked"];
	        this.locked = source["locked"];
	    }
	}
	export class AtlasSection {
	    id: string;
	    label: string;
	    items: AtlasItem[];
	    total: number;
	    summary: AtlasSectionSummary;
	
	    static createFrom(source: any = {}) {
	        return new AtlasSection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.items = this.convertValues(source["items"], AtlasItem);
	        this.total = source["total"];
	        this.summary = this.convertValues(source["summary"], AtlasSectionSummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AtlasPreviewPayload {
	    source: string;
	    status: string;
	    message: string;
	    refreshEnabled: boolean;
	    buyEnabled: boolean;
	    sections: AtlasSection[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new AtlasPreviewPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.refreshEnabled = source["refreshEnabled"];
	        this.buyEnabled = source["buyEnabled"];
	        this.sections = this.convertValues(source["sections"], AtlasSection);
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AtlasPurchasePlanSummary {
	    requested: number;
	    lockedCropCount: number;
	    purchasable: number;
	    skipped: number;
	    skipReasons: Record<string, number>;
	    countPerSeed: number;
	    effectiveLevel: number;
	
	    static createFrom(source: any = {}) {
	        return new AtlasPurchasePlanSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requested = source["requested"];
	        this.lockedCropCount = source["lockedCropCount"];
	        this.purchasable = source["purchasable"];
	        this.skipped = source["skipped"];
	        this.skipReasons = source["skipReasons"];
	        this.countPerSeed = source["countPerSeed"];
	        this.effectiveLevel = source["effectiveLevel"];
	    }
	}
	export class AtlasPurchaseSkipped {
	    name?: string;
	    seedId?: number;
	    fruitId?: number;
	    sort?: number;
	    reason: string;
	    requiredLevel?: number;
	    effectiveLevel?: number;
	
	    static createFrom(source: any = {}) {
	        return new AtlasPurchaseSkipped(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.seedId = source["seedId"];
	        this.fruitId = source["fruitId"];
	        this.sort = source["sort"];
	        this.reason = source["reason"];
	        this.requiredLevel = source["requiredLevel"];
	        this.effectiveLevel = source["effectiveLevel"];
	    }
	}
	export class AtlasSeedPurchase {
	    seedId: number;
	    seedName: string;
	    goodsId: number;
	    price: number;
	    count: number;
	    requiredLevel: number;
	    sort: number;
	
	    static createFrom(source: any = {}) {
	        return new AtlasSeedPurchase(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seedId = source["seedId"];
	        this.seedName = source["seedName"];
	        this.goodsId = source["goodsId"];
	        this.price = source["price"];
	        this.count = source["count"];
	        this.requiredLevel = source["requiredLevel"];
	        this.sort = source["sort"];
	    }
	}
	export class AtlasPurchasePlan {
	    purchases: AtlasSeedPurchase[];
	    skipped: AtlasPurchaseSkipped[];
	    summary: AtlasPurchasePlanSummary;
	
	    static createFrom(source: any = {}) {
	        return new AtlasPurchasePlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.purchases = this.convertValues(source["purchases"], AtlasSeedPurchase);
	        this.skipped = this.convertValues(source["skipped"], AtlasPurchaseSkipped);
	        this.summary = this.convertValues(source["summary"], AtlasPurchasePlanSummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AtlasPurchasePayload {
	    ok: boolean;
	    error?: string;
	    profile?: Record<string, any>;
	    level: AnalyticsLevelInfo;
	    plan: AtlasPurchasePlan;
	    purchase?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new AtlasPurchasePayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.error = source["error"];
	        this.profile = source["profile"];
	        this.level = this.convertValues(source["level"], AnalyticsLevelInfo);
	        this.plan = this.convertValues(source["plan"], AtlasPurchasePlan);
	        this.purchase = source["purchase"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class AtlasPurchasePreviewPayload {
	    ok: boolean;
	    error?: string;
	    profile?: Record<string, any>;
	    level: AnalyticsLevelInfo;
	    plan: AtlasPurchasePlan;
	
	    static createFrom(source: any = {}) {
	        return new AtlasPurchasePreviewPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.error = source["error"];
	        this.profile = source["profile"];
	        this.level = this.convertValues(source["level"], AnalyticsLevelInfo);
	        this.plan = this.convertValues(source["plan"], AtlasPurchasePlan);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	export class BackpackSeedOption {
	    seedId: number;
	    name: string;
	    level?: number;
	    count: number;
	    backpackCount: number;
	    inBackpack: boolean;
	    disabled: boolean;
	    plantable: boolean;
	    plantableReason?: string;
	    plantableMessage?: string;
	    plantSize: number;
	    retainedByForcePriority: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BackpackSeedOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seedId = source["seedId"];
	        this.name = source["name"];
	        this.level = source["level"];
	        this.count = source["count"];
	        this.backpackCount = source["backpackCount"];
	        this.inBackpack = source["inBackpack"];
	        this.disabled = source["disabled"];
	        this.plantable = source["plantable"];
	        this.plantableReason = source["plantableReason"];
	        this.plantableMessage = source["plantableMessage"];
	        this.plantSize = source["plantSize"];
	        this.retainedByForcePriority = source["retainedByForcePriority"];
	    }
	}
	export class BackpackSeedOptionsPayload {
	    ok: boolean;
	    updatedAt: number;
	    list: BackpackSeedOption[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new BackpackSeedOptionsPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.updatedAt = source["updatedAt"];
	        this.list = this.convertValues(source["list"], BackpackSeedOption);
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CropAnalyticsItem {
	    id: number;
	    seedId: number;
	    name: string;
	    seasons: number;
	    level?: number;
	    growTime: number;
	    growTimeText: string;
	    reduceSecApplied: number;
	    harvestExp: number;
	    expPerHour: number;
	    normalFertilizerExpPerHour: number;
	    income: number;
	    netProfit: number;
	    profitPerHour: number;
	    normalFertilizerProfitPerHour: number;
	    fruitId: number;
	    fruitCount: number;
	    fruitPrice: number;
	    seedPrice: number;
	    plantSize: number;
	    shopEligible: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CropAnalyticsItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.seedId = source["seedId"];
	        this.name = source["name"];
	        this.seasons = source["seasons"];
	        this.level = source["level"];
	        this.growTime = source["growTime"];
	        this.growTimeText = source["growTimeText"];
	        this.reduceSecApplied = source["reduceSecApplied"];
	        this.harvestExp = source["harvestExp"];
	        this.expPerHour = source["expPerHour"];
	        this.normalFertilizerExpPerHour = source["normalFertilizerExpPerHour"];
	        this.income = source["income"];
	        this.netProfit = source["netProfit"];
	        this.profitPerHour = source["profitPerHour"];
	        this.normalFertilizerProfitPerHour = source["normalFertilizerProfitPerHour"];
	        this.fruitId = source["fruitId"];
	        this.fruitCount = source["fruitCount"];
	        this.fruitPrice = source["fruitPrice"];
	        this.seedPrice = source["seedPrice"];
	        this.plantSize = source["plantSize"];
	        this.shopEligible = source["shopEligible"];
	    }
	}
	export class PlantRecommendationCard {
	    value: string;
	    label: string;
	    recommended?: CropAnalyticsItem;
	    currentRecommended?: CropAnalyticsItem;
	    currentSource?: string;
	    theoreticalRecommended?: CropAnalyticsItem;
	
	    static createFrom(source: any = {}) {
	        return new PlantRecommendationCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.label = source["label"];
	        this.recommended = this.convertValues(source["recommended"], CropAnalyticsItem);
	        this.currentRecommended = this.convertValues(source["currentRecommended"], CropAnalyticsItem);
	        this.currentSource = source["currentSource"];
	        this.theoreticalRecommended = this.convertValues(source["theoreticalRecommended"], CropAnalyticsItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PlantStrategyMode {
	    value: string;
	    label: string;
	    needsSeedId: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PlantStrategyMode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.label = source["label"];
	        this.needsSeedId = source["needsSeedId"];
	    }
	}
	export class CropAnalyticsPayload {
	    source: string;
	    items: CropAnalyticsItem[];
	    sort: string;
	    requestedMaxLevel: number;
	    effectiveMaxLevel: number;
	    levelSource: string;
	    profile?: Record<string, any>;
	    strategies: PlantStrategyMode[];
	    recommendations: PlantRecommendationCard[];
	    runtimeError?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new CropAnalyticsPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.items = this.convertValues(source["items"], CropAnalyticsItem);
	        this.sort = source["sort"];
	        this.requestedMaxLevel = source["requestedMaxLevel"];
	        this.effectiveMaxLevel = source["effectiveMaxLevel"];
	        this.levelSource = source["levelSource"];
	        this.profile = source["profile"];
	        this.strategies = this.convertValues(source["strategies"], PlantStrategyMode);
	        this.recommendations = this.convertValues(source["recommendations"], PlantRecommendationCard);
	        this.runtimeError = source["runtimeError"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class FarmFeature {
	    id: string;
	    label: string;
	    summary: string;
	    status: string;
	    reference: string;
	
	    static createFrom(source: any = {}) {
	        return new FarmFeature(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.summary = source["summary"];
	        this.status = source["status"];
	        this.reference = source["reference"];
	    }
	}
	export class FeatureGroup {
	    id: string;
	    label: string;
	    summary: string;
	    features: FarmFeature[];
	
	    static createFrom(source: any = {}) {
	        return new FeatureGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.summary = source["summary"];
	        this.features = this.convertValues(source["features"], FarmFeature);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FeatureCatalog {
	    groups: FeatureGroup[];
	
	    static createFrom(source: any = {}) {
	        return new FeatureCatalog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groups = this.convertValues(source["groups"], FeatureGroup);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class RuntimeActionGate {
	    id: string;
	    label: string;
	    enabled: boolean;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new RuntimeActionGate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.enabled = source["enabled"];
	        this.reason = source["reason"];
	    }
	}
	export class LandMutationTypeItem {
	    typeId?: number;
	    name: string;
	    iconUrl?: string;
	
	    static createFrom(source: any = {}) {
	        return new LandMutationTypeItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.typeId = source["typeId"];
	        this.name = source["name"];
	        this.iconUrl = source["iconUrl"];
	    }
	}
	export class LandDetailsItem {
	    id: string;
	    landId: number;
	    landLevel?: number;
	    plantId?: number;
	    seedId?: number;
	    landType?: string;
	    landTypeLabel?: string;
	    plantName?: string;
	    displayPlantName?: string;
	    imageUrl?: string;
	    status: string;
	    statusLabel: string;
	    matureInSec?: number;
	    matureAtMs?: number;
	    matureEtaText?: string;
	    currentSeason?: number;
	    totalSeason?: number;
	    currentStage?: number;
	    phaseName?: string;
	    isMultiSeason?: boolean;
	    landSize?: number;
	    occupancyPlantSize?: number;
	    occupancyAnchorLandId?: number;
	    occupiedByMultiTilePlant?: boolean;
	    hasMutation?: boolean;
	    mutationLabel?: string;
	    mutationIconUrl?: string;
	    mutationImageUrl?: string;
	    mutationTypes?: LandMutationTypeItem[];
	    needWater?: boolean;
	    needWeed?: boolean;
	    needBug?: boolean;
	    needGoldenBug?: boolean;
	    needEraseDead?: boolean;
	    canHarvest: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LandDetailsItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.landId = source["landId"];
	        this.landLevel = source["landLevel"];
	        this.plantId = source["plantId"];
	        this.seedId = source["seedId"];
	        this.landType = source["landType"];
	        this.landTypeLabel = source["landTypeLabel"];
	        this.plantName = source["plantName"];
	        this.displayPlantName = source["displayPlantName"];
	        this.imageUrl = source["imageUrl"];
	        this.status = source["status"];
	        this.statusLabel = source["statusLabel"];
	        this.matureInSec = source["matureInSec"];
	        this.matureAtMs = source["matureAtMs"];
	        this.matureEtaText = source["matureEtaText"];
	        this.currentSeason = source["currentSeason"];
	        this.totalSeason = source["totalSeason"];
	        this.currentStage = source["currentStage"];
	        this.phaseName = source["phaseName"];
	        this.isMultiSeason = source["isMultiSeason"];
	        this.landSize = source["landSize"];
	        this.occupancyPlantSize = source["occupancyPlantSize"];
	        this.occupancyAnchorLandId = source["occupancyAnchorLandId"];
	        this.occupiedByMultiTilePlant = source["occupiedByMultiTilePlant"];
	        this.hasMutation = source["hasMutation"];
	        this.mutationLabel = source["mutationLabel"];
	        this.mutationIconUrl = source["mutationIconUrl"];
	        this.mutationImageUrl = source["mutationImageUrl"];
	        this.mutationTypes = this.convertValues(source["mutationTypes"], LandMutationTypeItem);
	        this.needWater = source["needWater"];
	        this.needWeed = source["needWeed"];
	        this.needBug = source["needBug"];
	        this.needGoldenBug = source["needGoldenBug"];
	        this.needEraseDead = source["needEraseDead"];
	        this.canHarvest = source["canHarvest"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LandDetailsDeltaPayload {
	    full: boolean;
	    revision: string;
	    status?: string;
	    message?: string;
	    farmType?: string;
	    totalGrids?: number;
	    lands: LandDetailsItem[];
	    removedLandIds: number[];
	    actions?: RuntimeActionGate[];
	    runtimeError?: string;
	
	    static createFrom(source: any = {}) {
	        return new LandDetailsDeltaPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.full = source["full"];
	        this.revision = source["revision"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.farmType = source["farmType"];
	        this.totalGrids = source["totalGrids"];
	        this.lands = this.convertValues(source["lands"], LandDetailsItem);
	        this.removedLandIds = source["removedLandIds"];
	        this.actions = this.convertValues(source["actions"], RuntimeActionGate);
	        this.runtimeError = source["runtimeError"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class LandDetailsPayload {
	    status: string;
	    message: string;
	    revision?: string;
	    farmType?: string;
	    totalGrids?: number;
	    lands: LandDetailsItem[];
	    actions: RuntimeActionGate[];
	    runtimeError?: string;
	
	    static createFrom(source: any = {}) {
	        return new LandDetailsPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.message = source["message"];
	        this.revision = source["revision"];
	        this.farmType = source["farmType"];
	        this.totalGrids = source["totalGrids"];
	        this.lands = this.convertValues(source["lands"], LandDetailsItem);
	        this.actions = this.convertValues(source["actions"], RuntimeActionGate);
	        this.runtimeError = source["runtimeError"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	export class RunStatistics {
	    startedAt: string;
	    durationSeconds: number;
	    collect: number;
	    farm: number;
	    steal: number;
	    help: number;
	    mischief: number;
	    saleEstimate: number;
	    estimateReady: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RunStatistics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.startedAt = source["startedAt"];
	        this.durationSeconds = source["durationSeconds"];
	        this.collect = source["collect"];
	        this.farm = source["farm"];
	        this.steal = source["steal"];
	        this.help = source["help"];
	        this.mischief = source["mischief"];
	        this.saleEstimate = source["saleEstimate"];
	        this.estimateReady = source["estimateReady"];
	    }
	}
	
	
	export class StealCropOption {
	    plantId: number;
	    seedId?: number;
	    name: string;
	    level?: number;
	    imageUrl?: string;
	    sortGroup?: number;
	    sortOrder?: number;
	
	    static createFrom(source: any = {}) {
	        return new StealCropOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.plantId = source["plantId"];
	        this.seedId = source["seedId"];
	        this.name = source["name"];
	        this.level = source["level"];
	        this.imageUrl = source["imageUrl"];
	        this.sortGroup = source["sortGroup"];
	        this.sortOrder = source["sortOrder"];
	    }
	}
	export class StealCropOptionsPayload {
	    ok: boolean;
	    updatedAt: number;
	    list: StealCropOption[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new StealCropOptionsPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.updatedAt = source["updatedAt"];
	        this.list = this.convertValues(source["list"], StealCropOption);
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class WarehouseCategorySummary {
	    key: string;
	    label: string;
	    distinct: number;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new WarehouseCategorySummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	        this.distinct = source["distinct"];
	        this.count = source["count"];
	    }
	}
	export class WarehouseItem {
	    id: string;
	    itemId: number;
	    name: string;
	    count: number;
	    category: string;
	    categoryLabel: string;
	    imageUrl?: string;
	    canSell: boolean;
	    locked: boolean;
	    estimatedSellPrice: number;
	
	    static createFrom(source: any = {}) {
	        return new WarehouseItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.itemId = source["itemId"];
	        this.name = source["name"];
	        this.count = source["count"];
	        this.category = source["category"];
	        this.categoryLabel = source["categoryLabel"];
	        this.imageUrl = source["imageUrl"];
	        this.canSell = source["canSell"];
	        this.locked = source["locked"];
	        this.estimatedSellPrice = source["estimatedSellPrice"];
	    }
	}
	export class WarehouseSummary {
	    totalDistinct: number;
	    totalCount: number;
	    sellableDistinct: number;
	    sellableCount: number;
	    estimatedAllSellPrice: number;
	    categoryList?: WarehouseCategorySummary[];
	
	    static createFrom(source: any = {}) {
	        return new WarehouseSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalDistinct = source["totalDistinct"];
	        this.totalCount = source["totalCount"];
	        this.sellableDistinct = source["sellableDistinct"];
	        this.sellableCount = source["sellableCount"];
	        this.estimatedAllSellPrice = source["estimatedAllSellPrice"];
	        this.categoryList = this.convertValues(source["categoryList"], WarehouseCategorySummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class WarehousePayload {
	    status: string;
	    message: string;
	    items: WarehouseItem[];
	    actions: RuntimeActionGate[];
	    summary: WarehouseSummary;
	    runtimeError?: string;
	
	    static createFrom(source: any = {}) {
	        return new WarehousePayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.message = source["message"];
	        this.items = this.convertValues(source["items"], WarehouseItem);
	        this.actions = this.convertValues(source["actions"], RuntimeActionGate);
	        this.summary = this.convertValues(source["summary"], WarehouseSummary);
	        this.runtimeError = source["runtimeError"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class WarehouseSellPayload {
	    ok: boolean;
	    error?: string;
	    warehouse: WarehousePayload;
	    sell?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new WarehouseSellPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.error = source["error"];
	        this.warehouse = this.convertValues(source["warehouse"], WarehousePayload);
	        this.sell = source["sell"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace guard {
	
	export class ActionRequest {
	    action: string;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new ActionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.reason = source["reason"];
	    }
	}
	export class ActionResult {
	    ok: boolean;
	    status: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.status = source["status"];
	        this.error = source["error"];
	    }
	}
	export class HostCandidateEvidence {
	    kind: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new HostCandidateEvidence(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.value = source["value"];
	    }
	}
	export class HostProcessCandidate {
	    pid: number;
	    parentPid?: number;
	    processName: string;
	    executablePath?: string;
	    windowTitles: string[];
	    hwnds: number[];
	    confidence: string;
	    boundOwner?: string;
	    available: boolean;
	    evidence: HostCandidateEvidence[];
	
	    static createFrom(source: any = {}) {
	        return new HostProcessCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.parentPid = source["parentPid"];
	        this.processName = source["processName"];
	        this.executablePath = source["executablePath"];
	        this.windowTitles = source["windowTitles"];
	        this.hwnds = source["hwnds"];
	        this.confidence = source["confidence"];
	        this.boundOwner = source["boundOwner"];
	        this.available = source["available"];
	        this.evidence = this.convertValues(source["evidence"], HostCandidateEvidence);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class HostBinding {
	    owner: string;
	    pid: number;
	    parentPid?: number;
	    processName: string;
	    executablePath?: string;
	    windowTitles: string[];
	    hwnds: number[];
	    confidence: string;
	    boundAt: string;
	
	    static createFrom(source: any = {}) {
	        return new HostBinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.owner = source["owner"];
	        this.pid = source["pid"];
	        this.parentPid = source["parentPid"];
	        this.processName = source["processName"];
	        this.executablePath = source["executablePath"];
	        this.windowTitles = source["windowTitles"];
	        this.hwnds = source["hwnds"];
	        this.confidence = source["confidence"];
	        this.boundAt = source["boundAt"];
	    }
	}
	export class AutoBindOwnerResult {
	    owner: string;
	    status: string;
	    binding?: HostBinding;
	    candidates: HostProcessCandidate[];
	
	    static createFrom(source: any = {}) {
	        return new AutoBindOwnerResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.owner = source["owner"];
	        this.status = source["status"];
	        this.binding = this.convertValues(source["binding"], HostBinding);
	        this.candidates = this.convertValues(source["candidates"], HostProcessCandidate);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class HostLaunchResult {
	    owner: string;
	    platform: string;
	    status: string;
	    reason: string;
	    launchDispatched: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HostLaunchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.owner = source["owner"];
	        this.platform = source["platform"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	        this.launchDispatched = source["launchDispatched"];
	    }
	}
	
	export class HostRestartPreview {
	    owner: string;
	    allowed: boolean;
	    status: string;
	    reason: string;
	    binding?: HostBinding;
	    current?: HostProcessCandidate;
	
	    static createFrom(source: any = {}) {
	        return new HostRestartPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.owner = source["owner"];
	        this.allowed = source["allowed"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	        this.binding = this.convertValues(source["binding"], HostBinding);
	        this.current = this.convertValues(source["current"], HostProcessCandidate);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RestartEvent {
	    at: string;
	    runtimeTarget: string;
	    reason: string;
	    trigger: string;
	    ok: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new RestartEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.at = source["at"];
	        this.runtimeTarget = source["runtimeTarget"];
	        this.reason = source["reason"];
	        this.trigger = source["trigger"];
	        this.ok = source["ok"];
	        this.error = source["error"];
	    }
	}
	export class RestartResult {
	    owner: string;
	    status: string;
	    reason: string;
	    oldPid?: number;
	    stopped: boolean;
	    launchDispatched: boolean;
	    binding?: HostBinding;
	    candidates: HostProcessCandidate[];
	
	    static createFrom(source: any = {}) {
	        return new RestartResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.owner = source["owner"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	        this.oldPid = source["oldPid"];
	        this.stopped = source["stopped"];
	        this.launchDispatched = source["launchDispatched"];
	        this.binding = this.convertValues(source["binding"], HostBinding);
	        this.candidates = this.convertValues(source["candidates"], HostProcessCandidate);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RuntimeEvent {
	    name: string;
	    phase: string;
	    runtimeTarget?: string;
	    accountKey?: string;
	    gid?: string;
	    handled?: boolean;
	    via?: string;
	    firstDetectedAt?: number;
	    handledAt?: number;
	    remainingMs?: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new RuntimeEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.phase = source["phase"];
	        this.runtimeTarget = source["runtimeTarget"];
	        this.accountKey = source["accountKey"];
	        this.gid = source["gid"];
	        this.handled = source["handled"];
	        this.via = source["via"];
	        this.firstDetectedAt = source["firstDetectedAt"];
	        this.handledAt = source["handledAt"];
	        this.remainingMs = source["remainingMs"];
	        this.error = source["error"];
	    }
	}
	export class Settings {
	    enabled: boolean;
	    failureRecoveryEnabled: boolean;
	    timeoutThreshold: number;
	    monitorIntervalMs: number;
	    restartReconnectGraceSec: number;
	    maxRestartsPer10Min: number;
	    scheduledRestartEnabled: boolean;
	    scheduledRestartIntervalMin: number;
	    autoMinimizeAfterRestart: boolean;
	    networkReconnectEnabled: boolean;
	    networkReconnectIntervalMs: number;
	    networkRecoveryTimeoutMs: number;
	    otherPlaceLoginEnabled: boolean;
	    otherPlaceLoginIntervalMs: number;
	    otherPlaceLoginDelayMin: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.failureRecoveryEnabled = source["failureRecoveryEnabled"];
	        this.timeoutThreshold = source["timeoutThreshold"];
	        this.monitorIntervalMs = source["monitorIntervalMs"];
	        this.restartReconnectGraceSec = source["restartReconnectGraceSec"];
	        this.maxRestartsPer10Min = source["maxRestartsPer10Min"];
	        this.scheduledRestartEnabled = source["scheduledRestartEnabled"];
	        this.scheduledRestartIntervalMin = source["scheduledRestartIntervalMin"];
	        this.autoMinimizeAfterRestart = source["autoMinimizeAfterRestart"];
	        this.networkReconnectEnabled = source["networkReconnectEnabled"];
	        this.networkReconnectIntervalMs = source["networkReconnectIntervalMs"];
	        this.networkRecoveryTimeoutMs = source["networkRecoveryTimeoutMs"];
	        this.otherPlaceLoginEnabled = source["otherPlaceLoginEnabled"];
	        this.otherPlaceLoginIntervalMs = source["otherPlaceLoginIntervalMs"];
	        this.otherPlaceLoginDelayMin = source["otherPlaceLoginDelayMin"];
	    }
	}
	export class Status {
	    enabled: boolean;
	    armed: boolean;
	    armReason?: string;
	    phase: string;
	    runtimeTarget: string;
	    timeoutStreak: number;
	    threshold: number;
	    lastCheckAt?: string;
	    lastTimeoutAt?: string;
	    lastHealthyAt?: string;
	    lastRestartAt?: string;
	    reconnectGraceUntil?: string;
	    reconnectGraceRemainingMs: number;
	    lastReason?: string;
	    lastActionError?: string;
	    restartCountInWindow: number;
	    maxRestartsPerWindow: number;
	    circuitOpen: boolean;
	    actionMode?: string;
	    recentRestartReason?: string;
	    recentRestartEvents: RestartEvent[];
	    scheduledRestartEnabled: boolean;
	    scheduledRestartIntervalMin: number;
	    scheduledRestartNextAt?: string;
	    scheduledRestartRemainingMs: number;
	    scheduledRestartLastAt?: string;
	    scheduledRestartLastResult?: string;
	    scheduledRestartLastError?: string;
	    autoMinimizeAfterRestart: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.armed = source["armed"];
	        this.armReason = source["armReason"];
	        this.phase = source["phase"];
	        this.runtimeTarget = source["runtimeTarget"];
	        this.timeoutStreak = source["timeoutStreak"];
	        this.threshold = source["threshold"];
	        this.lastCheckAt = source["lastCheckAt"];
	        this.lastTimeoutAt = source["lastTimeoutAt"];
	        this.lastHealthyAt = source["lastHealthyAt"];
	        this.lastRestartAt = source["lastRestartAt"];
	        this.reconnectGraceUntil = source["reconnectGraceUntil"];
	        this.reconnectGraceRemainingMs = source["reconnectGraceRemainingMs"];
	        this.lastReason = source["lastReason"];
	        this.lastActionError = source["lastActionError"];
	        this.restartCountInWindow = source["restartCountInWindow"];
	        this.maxRestartsPerWindow = source["maxRestartsPerWindow"];
	        this.circuitOpen = source["circuitOpen"];
	        this.actionMode = source["actionMode"];
	        this.recentRestartReason = source["recentRestartReason"];
	        this.recentRestartEvents = this.convertValues(source["recentRestartEvents"], RestartEvent);
	        this.scheduledRestartEnabled = source["scheduledRestartEnabled"];
	        this.scheduledRestartIntervalMin = source["scheduledRestartIntervalMin"];
	        this.scheduledRestartNextAt = source["scheduledRestartNextAt"];
	        this.scheduledRestartRemainingMs = source["scheduledRestartRemainingMs"];
	        this.scheduledRestartLastAt = source["scheduledRestartLastAt"];
	        this.scheduledRestartLastResult = source["scheduledRestartLastResult"];
	        this.scheduledRestartLastError = source["scheduledRestartLastError"];
	        this.autoMinimizeAfterRestart = source["autoMinimizeAfterRestart"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class WorkerStatus {
	    enabled: boolean;
	    running: boolean;
	    busy: boolean;
	    lastCheckAt?: string;
	    lastHandledAt?: string;
	    lastResult?: string;
	    lastError?: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.running = source["running"];
	        this.busy = source["busy"];
	        this.lastCheckAt = source["lastCheckAt"];
	        this.lastHandledAt = source["lastHandledAt"];
	        this.lastResult = source["lastResult"];
	        this.lastError = source["lastError"];
	    }
	}

}

export namespace license {
	
	export class ConnectionResult {
	    id: string;
	    name: string;
	    dnsAddress?: string;
	    ipAddress?: string;
	    resolveMs?: number;
	    sent: number;
	    received: number;
	    averagePingMs?: number;
	    reachable: boolean;
	    errorCode?: string;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.dnsAddress = source["dnsAddress"];
	        this.ipAddress = source["ipAddress"];
	        this.resolveMs = source["resolveMs"];
	        this.sent = source["sent"];
	        this.received = source["received"];
	        this.averagePingMs = source["averagePingMs"];
	        this.reachable = source["reachable"];
	        this.errorCode = source["errorCode"];
	        this.message = source["message"];
	    }
	}
	export class ConnectionReport {
	    host: string;
	    checkedAt: string;
	    results: ConnectionResult[];
	    recommendedId?: string;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.checkedAt = source["checkedAt"];
	        this.results = this.convertValues(source["results"], ConnectionResult);
	        this.recommendedId = source["recommendedId"];
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class LoginRequest {
	    card: string;
	    remember: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LoginRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.card = source["card"];
	        this.remember = source["remember"];
	    }
	}
	export class ProgramNotice {
	    content: string;
	    errorCode?: string;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProgramNotice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.content = source["content"];
	        this.errorCode = source["errorCode"];
	        this.message = source["message"];
	    }
	}
	export class ProgramVersion {
	    number: number;
	    name: string;
	    description?: string;
	    downloadUrl?: string;
	    notice?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProgramVersion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.downloadUrl = source["downloadUrl"];
	        this.notice = source["notice"];
	    }
	}
	export class RememberedCard {
	    card: string;
	    remembered: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RememberedCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.card = source["card"];
	        this.remembered = source["remembered"];
	    }
	}
	export class RenewalRequest {
	    rechargeCard: string;
	
	    static createFrom(source: any = {}) {
	        return new RenewalRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rechargeCard = source["rechargeCard"];
	    }
	}
	export class RenewalResult {
	    renewed: boolean;
	    refreshed: boolean;
	    expireTime?: string;
	    errorCode?: string;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new RenewalResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.renewed = source["renewed"];
	        this.refreshed = source["refreshed"];
	        this.expireTime = source["expireTime"];
	        this.errorCode = source["errorCode"];
	        this.message = source["message"];
	    }
	}
	export class UserInfo {
	    id: string;
	    serverExpireTime: string;
	    serverRemainNum: number;
	    serverType: string;
	    trial: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UserInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.serverExpireTime = source["serverExpireTime"];
	        this.serverRemainNum = source["serverRemainNum"];
	        this.serverType = source["serverType"];
	        this.trial = source["trial"];
	    }
	}
	export class Status {
	    phase: string;
	    authorized: boolean;
	    generation: number;
	    message?: string;
	    errorCode?: string;
	    user: UserInfo;
	    heartbeatFailures: number;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.phase = source["phase"];
	        this.authorized = source["authorized"];
	        this.generation = source["generation"];
	        this.message = source["message"];
	        this.errorCode = source["errorCode"];
	        this.user = this.convertValues(source["user"], UserInfo);
	        this.heartbeatFailures = source["heartbeatFailures"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Version {
	    number: number;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new Version(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.name = source["name"];
	    }
	}
	export class UpdateState {
	    current: Version;
	    latest: ProgramVersion;
	    available: boolean;
	    errorCode?: string;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = this.convertValues(source["current"], Version);
	        this.latest = this.convertValues(source["latest"], ProgramVersion);
	        this.available = source["available"];
	        this.errorCode = source["errorCode"];
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace desktop {
	
	export class GuardianStatusDTO {
	    enabled: boolean;
	    running: boolean;
	    phase: string;
	    runtimeTarget: string;
	    settings: guard.Settings;
	    process: guard.Status;
	    network: guard.WorkerStatus;
	    otherPlaceLogin: guard.WorkerStatus;
	    recentEvents: guard.RuntimeEvent[];
	
	    static createFrom(source: any = {}) {
	        return new GuardianStatusDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.running = source["running"];
	        this.phase = source["phase"];
	        this.runtimeTarget = source["runtimeTarget"];
	        this.settings = this.convertValues(source["settings"], guard.Settings);
	        this.process = this.convertValues(source["process"], guard.Status);
	        this.network = this.convertValues(source["network"], guard.WorkerStatus);
	        this.otherPlaceLogin = this.convertValues(source["otherPlaceLogin"], guard.WorkerStatus);
	        this.recentEvents = this.convertValues(source["recentEvents"], guard.RuntimeEvent);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LANAccessSettingsInput {
	    enabled: boolean;
	    mode: string;
	    port: number;
	    password: string;
	    confirmPassword: string;
	
	    static createFrom(source: any = {}) {
	        return new LANAccessSettingsInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.mode = source["mode"];
	        this.port = source["port"];
	        this.password = source["password"];
	        this.confirmPassword = source["confirmPassword"];
	    }
	}
	export class LANAccessStatus {
	    enabled: boolean;
	    mode: string;
	    port: number;
	    passwordConfigured: boolean;
	    running: boolean;
	    address: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new LANAccessStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.mode = source["mode"];
	        this.port = source["port"];
	        this.passwordConfigured = source["passwordConfigured"];
	        this.running = source["running"];
	        this.address = source["address"];
	        this.error = source["error"];
	    }
	}
	export class QQDebugPatchStatus {
	    phase: string;
	    injected: boolean;
	    inFlight: boolean;
	    lastTrigger?: string;
	    action?: string;
	    targetPath?: string;
	    backupPath?: string;
	    scriptHash?: string;
	    hostVersion?: string;
	    candidatePaths?: string[];
	    restartRequired: boolean;
	    error?: string;
	    startedAt?: string;
	    finishedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new QQDebugPatchStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.phase = source["phase"];
	        this.injected = source["injected"];
	        this.inFlight = source["inFlight"];
	        this.lastTrigger = source["lastTrigger"];
	        this.action = source["action"];
	        this.targetPath = source["targetPath"];
	        this.backupPath = source["backupPath"];
	        this.scriptHash = source["scriptHash"];
	        this.hostVersion = source["hostVersion"];
	        this.candidatePaths = source["candidatePaths"];
	        this.restartRequired = source["restartRequired"];
	        this.error = source["error"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	    }
	}
	export class RuntimeAccount {
	    accountKey: string;
	    gid: number;
	    nickname?: string;
	    avatarUrl?: string;
	    confirmed: boolean;
	    identifiedAt?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new RuntimeAccount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountKey = source["accountKey"];
	        this.gid = source["gid"];
	        this.nickname = source["nickname"];
	        this.avatarUrl = source["avatarUrl"];
	        this.confirmed = source["confirmed"];
	        this.identifiedAt = source["identifiedAt"];
	        this.error = source["error"];
	    }
	}
	export class SaveTextFileFilter {
	    name: string;
	    extensions: string[];
	
	    static createFrom(source: any = {}) {
	        return new SaveTextFileFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.extensions = source["extensions"];
	    }
	}
	export class SaveTextFileRequest {
	    defaultName: string;
	    content: string;
	    filters?: SaveTextFileFilter[];
	
	    static createFrom(source: any = {}) {
	        return new SaveTextFileRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultName = source["defaultName"];
	        this.content = source["content"];
	        this.filters = this.convertValues(source["filters"], SaveTextFileFilter);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SaveTextFileResult {
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new SaveTextFileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	    }
	}

}

export namespace maintenance {
	
	export class Target {
	    path: string;
	    relativePath: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new Target(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.relativePath = source["relativePath"];
	        this.reason = source["reason"];
	    }
	}
	export class Summary {
	    platform: string;
	    preview: boolean;
	    targetCount: number;
	    movedCount: number;
	    skippedCount: number;
	    closedProcesses: number;
	    backupDir: string;
	    targets: Target[];
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.platform = source["platform"];
	        this.preview = source["preview"];
	        this.targetCount = source["targetCount"];
	        this.movedCount = source["movedCount"];
	        this.skippedCount = source["skippedCount"];
	        this.closedProcesses = source["closedProcesses"];
	        this.backupDir = source["backupDir"];
	        this.targets = this.convertValues(source["targets"], Target);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace messagepush {
	
	export class ChannelMeta {
	    type: string;
	    label: string;
	    configured: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ChannelMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.label = source["label"];
	        this.configured = source["configured"];
	    }
	}
	export class ChannelMode {
	    channel: string;
	    modes: string[];
	
	    static createFrom(source: any = {}) {
	        return new ChannelMode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channel = source["channel"];
	        this.modes = source["modes"];
	    }
	}
	export class ChannelResult {
	    type: string;
	    ok: boolean;
	    error?: string;
	    attempts: number;
	
	    static createFrom(source: any = {}) {
	        return new ChannelResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.ok = source["ok"];
	        this.error = source["error"];
	        this.attempts = source["attempts"];
	    }
	}
	export class Channels {
	    serverChanSendKey: string;
	    pushPlusToken: string;
	    qmsgKey: string;
	    qmsgType: string;
	    qmsgTarget: string;
	    wecomWebhook: string;
	    dingtalkWebhook: string;
	    dingtalkSecret: string;
	    feishuWebhook: string;
	    telegramBotToken: string;
	    telegramChatId: string;
	    barkServerUrl: string;
	    barkDeviceKey: string;
	    ntfyServerUrl: string;
	    ntfyTopic: string;
	    webhookUrl: string;
	    webhookMethod: string;
	    webhookHeaders: string;
	
	    static createFrom(source: any = {}) {
	        return new Channels(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serverChanSendKey = source["serverChanSendKey"];
	        this.pushPlusToken = source["pushPlusToken"];
	        this.qmsgKey = source["qmsgKey"];
	        this.qmsgType = source["qmsgType"];
	        this.qmsgTarget = source["qmsgTarget"];
	        this.wecomWebhook = source["wecomWebhook"];
	        this.dingtalkWebhook = source["dingtalkWebhook"];
	        this.dingtalkSecret = source["dingtalkSecret"];
	        this.feishuWebhook = source["feishuWebhook"];
	        this.telegramBotToken = source["telegramBotToken"];
	        this.telegramChatId = source["telegramChatId"];
	        this.barkServerUrl = source["barkServerUrl"];
	        this.barkDeviceKey = source["barkDeviceKey"];
	        this.ntfyServerUrl = source["ntfyServerUrl"];
	        this.ntfyTopic = source["ntfyTopic"];
	        this.webhookUrl = source["webhookUrl"];
	        this.webhookMethod = source["webhookMethod"];
	        this.webhookHeaders = source["webhookHeaders"];
	    }
	}
	export class LogMonitorRule {
	    enabled: boolean;
	    source: string;
	    type: string;
	    keyword: string;
	
	    static createFrom(source: any = {}) {
	        return new LogMonitorRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.source = source["source"];
	        this.type = source["type"];
	        this.keyword = source["keyword"];
	    }
	}
	export class Config {
	    enabled: boolean;
	    abnormalEnabled: boolean;
	    suspectedEnabled: boolean;
	    recoveryEnabled: boolean;
	    dailyEnabled: boolean;
	    dailyMarkdownCardEnabled: boolean;
	    restartEnabled: boolean;
	    dailyTime: string;
	    logMonitorEnabled: boolean;
	    abnormalTimeoutThreshold: number;
	    logScanIntervalSec: number;
	    httpTimeoutMs: number;
	    pushRetryCount: number;
	    selectedChannels: string[];
	    channelFormats: Record<string, string>;
	    channels: Channels;
	    templates: Record<string, any>;
	    logMonitorRules: LogMonitorRule[];
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.abnormalEnabled = source["abnormalEnabled"];
	        this.suspectedEnabled = source["suspectedEnabled"];
	        this.recoveryEnabled = source["recoveryEnabled"];
	        this.dailyEnabled = source["dailyEnabled"];
	        this.dailyMarkdownCardEnabled = source["dailyMarkdownCardEnabled"];
	        this.restartEnabled = source["restartEnabled"];
	        this.dailyTime = source["dailyTime"];
	        this.logMonitorEnabled = source["logMonitorEnabled"];
	        this.abnormalTimeoutThreshold = source["abnormalTimeoutThreshold"];
	        this.logScanIntervalSec = source["logScanIntervalSec"];
	        this.httpTimeoutMs = source["httpTimeoutMs"];
	        this.pushRetryCount = source["pushRetryCount"];
	        this.selectedChannels = source["selectedChannels"];
	        this.channelFormats = source["channelFormats"];
	        this.channels = this.convertValues(source["channels"], Channels);
	        this.templates = source["templates"];
	        this.logMonitorRules = this.convertValues(source["logMonitorRules"], LogMonitorRule);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DailyState {
	    enabled: boolean;
	    channels: string[];
	    time: string;
	    nextRunAt?: string;
	    lastSummaryDateKey?: string;
	    lastSummaryAt?: string;
	    lastCheckedAt?: string;
	    lastSkipReason?: string;
	
	    static createFrom(source: any = {}) {
	        return new DailyState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.channels = source["channels"];
	        this.time = source["time"];
	        this.nextRunAt = source["nextRunAt"];
	        this.lastSummaryDateKey = source["lastSummaryDateKey"];
	        this.lastSummaryAt = source["lastSummaryAt"];
	        this.lastCheckedAt = source["lastCheckedAt"];
	        this.lastSkipReason = source["lastSkipReason"];
	    }
	}
	
	export class Payload {
	    kind: string;
	    title: string;
	    lines: string[];
	    meta?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Payload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.lines = source["lines"];
	        this.meta = source["meta"];
	    }
	}
	export class PushRecord {
	    time: string;
	    kind: string;
	    title: string;
	    ok: boolean;
	    channels: string[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PushRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.ok = source["ok"];
	        this.channels = source["channels"];
	        this.error = source["error"];
	    }
	}
	export class TemplateContext {
	    MessageType: string;
	    Values: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new TemplateContext(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.MessageType = source["MessageType"];
	        this.Values = source["Values"];
	    }
	}
	export class RenderedPayload {
	    Kind: string;
	    Mode: string;
	    Text: string;
	    JSON: Record<string, any>;
	    Context: TemplateContext;
	
	    static createFrom(source: any = {}) {
	        return new RenderedPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Kind = source["Kind"];
	        this.Mode = source["Mode"];
	        this.Text = source["Text"];
	        this.JSON = source["JSON"];
	        this.Context = this.convertValues(source["Context"], TemplateContext);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SendResult {
	    ok: boolean;
	    summary: string;
	    results: ChannelResult[];
	    payload: Payload;
	
	    static createFrom(source: any = {}) {
	        return new SendResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.summary = source["summary"];
	        this.results = this.convertValues(source["results"], ChannelResult);
	        this.payload = this.convertValues(source["payload"], Payload);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Template {
	    enabled: boolean;
	    mode: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new Template(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.mode = source["mode"];
	        this.content = source["content"];
	    }
	}
	
	export class TemplateVariable {
	    path: string;
	    label: string;
	    description: string;
	    example: string;
	
	    static createFrom(source: any = {}) {
	        return new TemplateVariable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.label = source["label"];
	        this.description = source["description"];
	        this.example = source["example"];
	    }
	}
	export class TemplateDefinition {
	    type: string;
	    label: string;
	    channelModes: ChannelMode[];
	    variables: TemplateVariable[];
	
	    static createFrom(source: any = {}) {
	        return new TemplateDefinition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.label = source["label"];
	        this.channelModes = this.convertValues(source["channelModes"], ChannelMode);
	        this.variables = this.convertValues(source["variables"], TemplateVariable);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TemplatePreview {
	    ok: boolean;
	    error?: string;
	    messageType: string;
	    channel: string;
	    rendered: RenderedPayload;
	
	    static createFrom(source: any = {}) {
	        return new TemplatePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.error = source["error"];
	        this.messageType = source["messageType"];
	        this.channel = source["channel"];
	        this.rendered = this.convertValues(source["rendered"], RenderedPayload);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ViewState {
	    enabled: boolean;
	    config: Config;
	    configuredChannels: string[];
	    channels: ChannelMeta[];
	    daily: DailyState;
	    recentPushes: PushRecord[];
	    logMonitorEnabled: boolean;
	    templateCatalog: TemplateDefinition[];
	
	    static createFrom(source: any = {}) {
	        return new ViewState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.config = this.convertValues(source["config"], Config);
	        this.configuredChannels = source["configuredChannels"];
	        this.channels = this.convertValues(source["channels"], ChannelMeta);
	        this.daily = this.convertValues(source["daily"], DailyState);
	        this.recentPushes = this.convertValues(source["recentPushes"], PushRecord);
	        this.logMonitorEnabled = source["logMonitorEnabled"];
	        this.templateCatalog = this.convertValues(source["templateCatalog"], TemplateDefinition);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace qqpatch {
	
	export class Result {
	    ok: boolean;
	    action: string;
	    targetPath?: string;
	    targetPaths?: string[];
	    backupPath?: string;
	    backupPaths?: string[];
	    scriptHash?: string;
	    hostVersion?: string;
	    candidatePaths?: string[];
	    restartRequired: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.action = source["action"];
	        this.targetPath = source["targetPath"];
	        this.targetPaths = source["targetPaths"];
	        this.backupPath = source["backupPath"];
	        this.backupPaths = source["backupPaths"];
	        this.scriptHash = source["scriptHash"];
	        this.hostVersion = source["hostVersion"];
	        this.candidatePaths = source["candidatePaths"];
	        this.restartRequired = source["restartRequired"];
	        this.error = source["error"];
	    }
	}

}

export namespace runtime {
	
	export class Status {
	    target: string;
	    phase: string;
	    connected: boolean;
	    ready: boolean;
	    instanceId?: string;
	    hostVersion?: string;
	    lastSeenAt?: string;
	    progressDetail?: string;
	    lastError?: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.phase = source["phase"];
	        this.connected = source["connected"];
	        this.ready = source["ready"];
	        this.instanceId = source["instanceId"];
	        this.hostVersion = source["hostVersion"];
	        this.lastSeenAt = source["lastSeenAt"];
	        this.progressDetail = source["progressDetail"];
	        this.lastError = source["lastError"];
	    }
	}

}

export namespace social {
	
	export class ActionResult {
	    ok: boolean;
	    status: string;
	    message: string;
	    data?: any;
	
	    static createFrom(source: any = {}) {
	        return new ActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.data = source["data"];
	    }
	}
	export class DogGuardActionRequest {
	    action: string;
	    refresh?: boolean;
	    skipScanned?: boolean;
	    excludeGuardDog?: boolean;
	    scanIntervalMs?: number;
	
	    static createFrom(source: any = {}) {
	        return new DogGuardActionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.refresh = source["refresh"];
	        this.skipScanned = source["skipScanned"];
	        this.excludeGuardDog = source["excludeGuardDog"];
	        this.scanIntervalMs = source["scanIntervalMs"];
	    }
	}
	export class DogGuardRow {
	    gid: number;
	    name: string;
	    displayName?: string;
	    remark?: string;
	    level?: number;
	    scanned: boolean;
	    hasGuardDog: boolean;
	    dogId?: number;
	    dogName?: string;
	    dogConfig?: Record<string, any>;
	    error?: string;
	    scannedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new DogGuardRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gid = source["gid"];
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.remark = source["remark"];
	        this.level = source["level"];
	        this.scanned = source["scanned"];
	        this.hasGuardDog = source["hasGuardDog"];
	        this.dogId = source["dogId"];
	        this.dogName = source["dogName"];
	        this.dogConfig = source["dogConfig"];
	        this.error = source["error"];
	        this.scannedAt = source["scannedAt"];
	    }
	}
	export class DogGuardState {
	    running: boolean;
	    stopRequested: boolean;
	    startedAt?: string;
	    finishedAt?: string;
	    total: number;
	    scanned: number;
	    hasGuardDogCount: number;
	    current?: DogGuardRow;
	    results: DogGuardRow[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new DogGuardState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.stopRequested = source["stopRequested"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.total = source["total"];
	        this.scanned = source["scanned"];
	        this.hasGuardDogCount = source["hasGuardDogCount"];
	        this.current = this.convertValues(source["current"], DogGuardRow);
	        this.results = this.convertValues(source["results"], DogGuardRow);
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ExportRequest {
	    groups: string[];
	    refreshFriendSnapshot?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ExportRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groups = source["groups"];
	        this.refreshFriendSnapshot = source["refreshFriendSnapshot"];
	    }
	}
	export class FriendRules {
	    whitelistEnabled: boolean;
	    whitelistScopes: string[];
	    whitelist: string[];
	    blacklistEnabled: boolean;
	    blacklistScopes: string[];
	    blacklist: string[];
	    maskedBlacklist: boolean;
	    maskedMaxLevel: number;
	
	    static createFrom(source: any = {}) {
	        return new FriendRules(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.whitelistEnabled = source["whitelistEnabled"];
	        this.whitelistScopes = source["whitelistScopes"];
	        this.whitelist = source["whitelist"];
	        this.blacklistEnabled = source["blacklistEnabled"];
	        this.blacklistScopes = source["blacklistScopes"];
	        this.blacklist = source["blacklist"];
	        this.maskedBlacklist = source["maskedBlacklist"];
	        this.maskedMaxLevel = source["maskedMaxLevel"];
	    }
	}
	export class FriendActionRequest {
	    action: string;
	    target?: string;
	    targets?: string[];
	    rules?: FriendRules;
	    dryRun?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FriendActionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.target = source["target"];
	        this.targets = source["targets"];
	        this.rules = this.convertValues(source["rules"], FriendRules);
	        this.dryRun = source["dryRun"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FriendRow {
	    gid: number;
	    name?: string;
	    displayName: string;
	    remark?: string;
	    avatarUrl?: string;
	    level?: number;
	    workCounts: Record<string, number>;
	    stealable: boolean;
	    helpable: boolean;
	    mischiefable: boolean;
	    blacklisted: boolean;
	    whitelisted: boolean;
	    maskedBlocked: boolean;
	    protected: boolean;
	    protocolBlocked: boolean;
	    hasGuardDog: boolean;
	    raw?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new FriendRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gid = source["gid"];
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.remark = source["remark"];
	        this.avatarUrl = source["avatarUrl"];
	        this.level = source["level"];
	        this.workCounts = source["workCounts"];
	        this.stealable = source["stealable"];
	        this.helpable = source["helpable"];
	        this.mischiefable = source["mischiefable"];
	        this.blacklisted = source["blacklisted"];
	        this.whitelisted = source["whitelisted"];
	        this.maskedBlocked = source["maskedBlocked"];
	        this.protected = source["protected"];
	        this.protocolBlocked = source["protocolBlocked"];
	        this.hasGuardDog = source["hasGuardDog"];
	        this.raw = source["raw"];
	    }
	}
	
	export class VisitorRecord {
	    id: string;
	    playerId: number;
	    name?: string;
	    displayName: string;
	    avatarUrl?: string;
	    actionType: number;
	    time: number;
	    stealItemId?: number;
	    stealItemName?: string;
	    stealItemNum?: number;
	    count?: number;
	    level?: number;
	    authorizedStatus?: number;
	    hostType?: number;
	    actionLabel: string;
	
	    static createFrom(source: any = {}) {
	        return new VisitorRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.playerId = source["playerId"];
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.avatarUrl = source["avatarUrl"];
	        this.actionType = source["actionType"];
	        this.time = source["time"];
	        this.stealItemId = source["stealItemId"];
	        this.stealItemName = source["stealItemName"];
	        this.stealItemNum = source["stealItemNum"];
	        this.count = source["count"];
	        this.level = source["level"];
	        this.authorizedStatus = source["authorizedStatus"];
	        this.hostType = source["hostType"];
	        this.actionLabel = source["actionLabel"];
	    }
	}
	export class StealItem {
	    itemId: number;
	    name: string;
	    count: number;
	    landIds?: number[];
	
	    static createFrom(source: any = {}) {
	        return new StealItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.itemId = source["itemId"];
	        this.name = source["name"];
	        this.count = source["count"];
	        this.landIds = source["landIds"];
	    }
	}
	export class StealRecord {
	    id: string;
	    gid: number;
	    name?: string;
	    displayName: string;
	    avatarUrl?: string;
	    level?: number;
	    stealCount: number;
	    action: string;
	    occurredAt: string;
	    landIds?: number[];
	    items?: StealItem[];
	    raw?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new StealRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.gid = source["gid"];
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	        this.avatarUrl = source["avatarUrl"];
	        this.level = source["level"];
	        this.stealCount = source["stealCount"];
	        this.action = source["action"];
	        this.occurredAt = source["occurredAt"];
	        this.landIds = source["landIds"];
	        this.items = this.convertValues(source["items"], StealItem);
	        this.raw = source["raw"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ImportExportPayload {
	    ok: boolean;
	    status: string;
	    message: string;
	    version: number;
	    exportedAt?: string;
	    accountKey: string;
	    groups: string[];
	    rules?: FriendRules;
	    friendSnapshot?: FriendRow[];
	    stealRecords?: StealRecord[];
	    visitorRecords?: VisitorRecord[];
	    dogGuard?: DogGuardState;
	
	    static createFrom(source: any = {}) {
	        return new ImportExportPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.version = source["version"];
	        this.exportedAt = source["exportedAt"];
	        this.accountKey = source["accountKey"];
	        this.groups = source["groups"];
	        this.rules = this.convertValues(source["rules"], FriendRules);
	        this.friendSnapshot = this.convertValues(source["friendSnapshot"], FriendRow);
	        this.stealRecords = this.convertValues(source["stealRecords"], StealRecord);
	        this.visitorRecords = this.convertValues(source["visitorRecords"], VisitorRecord);
	        this.dogGuard = this.convertValues(source["dogGuard"], DogGuardState);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProtocolBlockList {
	    ok: boolean;
	    status: string;
	    message: string;
	    friends: FriendRow[];
	    raw?: any;
	
	    static createFrom(source: any = {}) {
	        return new ProtocolBlockList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.friends = this.convertValues(source["friends"], FriendRow);
	        this.raw = source["raw"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RankingPageRow {
	    kind: string;
	    key: string;
	    timeMS: number;
	    displayName: string;
	    rank?: number;
	    eventCount?: number;
	    stealCount?: number;
	    items: StealItem[];
	    actionType?: number;
	    actionLabel?: string;
	    actionTarget?: string;
	
	    static createFrom(source: any = {}) {
	        return new RankingPageRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.key = source["key"];
	        this.timeMS = source["timeMS"];
	        this.displayName = source["displayName"];
	        this.rank = source["rank"];
	        this.eventCount = source["eventCount"];
	        this.stealCount = source["stealCount"];
	        this.items = this.convertValues(source["items"], StealItem);
	        this.actionType = source["actionType"];
	        this.actionLabel = source["actionLabel"];
	        this.actionTarget = source["actionTarget"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RankingSummary {
	    visitorCount: number;
	    stolenFromMeCount: number;
	    stolenByMeCount: number;
	    stolenByMeRecordCount: number;
	
	    static createFrom(source: any = {}) {
	        return new RankingSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.visitorCount = source["visitorCount"];
	        this.stolenFromMeCount = source["stolenFromMeCount"];
	        this.stolenByMeCount = source["stolenByMeCount"];
	        this.stolenByMeRecordCount = source["stolenByMeRecordCount"];
	    }
	}
	export class RankingPage {
	    ok: boolean;
	    status: string;
	    message: string;
	    tab: string;
	    viewMode: string;
	    dateRange: string;
	    summary: RankingSummary;
	    rows: RankingPageRow[];
	    nextCursor?: string;
	    hasMore: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RankingPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.tab = source["tab"];
	        this.viewMode = source["viewMode"];
	        this.dateRange = source["dateRange"];
	        this.summary = this.convertValues(source["summary"], RankingSummary);
	        this.rows = this.convertValues(source["rows"], RankingPageRow);
	        this.nextCursor = source["nextCursor"];
	        this.hasMore = source["hasMore"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class RankingPreferences {
	    stolenByMeViewMode: string;
	    stolenFromMeViewMode: string;
	
	    static createFrom(source: any = {}) {
	        return new RankingPreferences(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stolenByMeViewMode = source["stolenByMeViewMode"];
	        this.stolenFromMeViewMode = source["stolenFromMeViewMode"];
	    }
	}
	export class RankingRequest {
	    tab?: string;
	    viewMode?: string;
	    dateRange?: string;
	    cursor?: string;
	    limit?: number;
	
	    static createFrom(source: any = {}) {
	        return new RankingRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tab = source["tab"];
	        this.viewMode = source["viewMode"];
	        this.dateRange = source["dateRange"];
	        this.cursor = source["cursor"];
	        this.limit = source["limit"];
	    }
	}
	
	export class Summary {
	    totalFriends: number;
	    stealableFriends: number;
	    helpableFriends: number;
	    mischiefFriends: number;
	    blacklisted: number;
	    whitelisted: number;
	    maskedBlocked: number;
	    protected: number;
	    dogGuardCount: number;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalFriends = source["totalFriends"];
	        this.stealableFriends = source["stealableFriends"];
	        this.helpableFriends = source["helpableFriends"];
	        this.mischiefFriends = source["mischiefFriends"];
	        this.blacklisted = source["blacklisted"];
	        this.whitelisted = source["whitelisted"];
	        this.maskedBlocked = source["maskedBlocked"];
	        this.protected = source["protected"];
	        this.dogGuardCount = source["dogGuardCount"];
	    }
	}
	export class State {
	    ok: boolean;
	    status: string;
	    message: string;
	    summary: Summary;
	    rules: FriendRules;
	    friends: FriendRow[];
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.summary = this.convertValues(source["summary"], Summary);
	        this.rules = this.convertValues(source["rules"], FriendRules);
	        this.friends = this.convertValues(source["friends"], FriendRow);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	

}

export namespace storage {
	
	export class MysteryShopPurchaseRecord {
	    id: string;
	    occurredAt: string;
	    goodsId: number;
	    itemId: number;
	    itemName: string;
	    count: number;
	    unitPrice: number;
	    currencyId: number;
	    currencyName: string;
	    discount: number;
	    payload?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new MysteryShopPurchaseRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.occurredAt = source["occurredAt"];
	        this.goodsId = source["goodsId"];
	        this.itemId = source["itemId"];
	        this.itemName = source["itemName"];
	        this.count = source["count"];
	        this.unitPrice = source["unitPrice"];
	        this.currencyId = source["currencyId"];
	        this.currencyName = source["currencyName"];
	        this.discount = source["discount"];
	        this.payload = source["payload"];
	    }
	}
	export class RuntimeSettings {
	    defaultTarget: string;
	    currentTarget: string;
	    autoStart: boolean;
	    cdpPort: number;
	    wmpfDebugPort: number;
	    processGuardEnabled: boolean;
	    processGuardFailureRecoveryEnabled: boolean;
	    processGuardTimeoutThreshold: number;
	    processGuardMonitorIntervalMs: number;
	    processGuardRestartReconnectGraceSec: number;
	    processGuardMaxRestartsPer10Min: number;
	    processGuardScheduledRestartEnabled: boolean;
	    processGuardScheduledRestartIntervalMin: number;
	    processGuardAutoMinimizeAfterRestart: boolean;
	    networkReconnectEnabled: boolean;
	    networkReconnectIntervalMs: number;
	    networkReconnectRecoveryTimeoutMs: number;
	    otherPlaceLoginReconnectEnabled: boolean;
	    otherPlaceLoginCheckIntervalMs: number;
	    otherPlaceLoginReconnectDelayMin: number;
	    autoWarehouseSellEnabled: boolean;
	    autoWarehouseSellIntervalMinute: number;
	    autoWarehouseSellCategories: string[];
	    warehouseRefreshOnlyOnAutoSell: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RuntimeSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultTarget = source["defaultTarget"];
	        this.currentTarget = source["currentTarget"];
	        this.autoStart = source["autoStart"];
	        this.cdpPort = source["cdpPort"];
	        this.wmpfDebugPort = source["wmpfDebugPort"];
	        this.processGuardEnabled = source["processGuardEnabled"];
	        this.processGuardFailureRecoveryEnabled = source["processGuardFailureRecoveryEnabled"];
	        this.processGuardTimeoutThreshold = source["processGuardTimeoutThreshold"];
	        this.processGuardMonitorIntervalMs = source["processGuardMonitorIntervalMs"];
	        this.processGuardRestartReconnectGraceSec = source["processGuardRestartReconnectGraceSec"];
	        this.processGuardMaxRestartsPer10Min = source["processGuardMaxRestartsPer10Min"];
	        this.processGuardScheduledRestartEnabled = source["processGuardScheduledRestartEnabled"];
	        this.processGuardScheduledRestartIntervalMin = source["processGuardScheduledRestartIntervalMin"];
	        this.processGuardAutoMinimizeAfterRestart = source["processGuardAutoMinimizeAfterRestart"];
	        this.networkReconnectEnabled = source["networkReconnectEnabled"];
	        this.networkReconnectIntervalMs = source["networkReconnectIntervalMs"];
	        this.networkReconnectRecoveryTimeoutMs = source["networkReconnectRecoveryTimeoutMs"];
	        this.otherPlaceLoginReconnectEnabled = source["otherPlaceLoginReconnectEnabled"];
	        this.otherPlaceLoginCheckIntervalMs = source["otherPlaceLoginCheckIntervalMs"];
	        this.otherPlaceLoginReconnectDelayMin = source["otherPlaceLoginReconnectDelayMin"];
	        this.autoWarehouseSellEnabled = source["autoWarehouseSellEnabled"];
	        this.autoWarehouseSellIntervalMinute = source["autoWarehouseSellIntervalMinute"];
	        this.autoWarehouseSellCategories = source["autoWarehouseSellCategories"];
	        this.warehouseRefreshOnlyOnAutoSell = source["warehouseRefreshOnlyOnAutoSell"];
	    }
	}
	export class UpdateCheckPreferences {
	    enabled: boolean;
	    intervalMinutes: number;
	
	    static createFrom(source: any = {}) {
	        return new UpdateCheckPreferences(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.intervalMinutes = source["intervalMinutes"];
	    }
	}
	export class WarehouseAutoSellSettings {
	    enabled: boolean;
	    intervalMinute: number;
	    categories: string[];
	    refreshOnlyOnAutoSell: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WarehouseAutoSellSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.intervalMinute = source["intervalMinute"];
	        this.categories = source["categories"];
	        this.refreshOnlyOnAutoSell = source["refreshOnlyOnAutoSell"];
	    }
	}
	export class WarehouseSellRecordItem {
	    itemId: number;
	    name?: string;
	    count: number;
	    unitPrice?: number;
	    amount: number;
	
	    static createFrom(source: any = {}) {
	        return new WarehouseSellRecordItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.itemId = source["itemId"];
	        this.name = source["name"];
	        this.count = source["count"];
	        this.unitPrice = source["unitPrice"];
	        this.amount = source["amount"];
	    }
	}
	export class WarehouseSellRecord {
	    id: string;
	    dateKey: string;
	    occurredAt: string;
	    mode: string;
	    itemKinds: number;
	    totalCount: number;
	    totalAmount: number;
	    items: WarehouseSellRecordItem[];
	    payload?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new WarehouseSellRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.dateKey = source["dateKey"];
	        this.occurredAt = source["occurredAt"];
	        this.mode = source["mode"];
	        this.itemKinds = source["itemKinds"];
	        this.totalCount = source["totalCount"];
	        this.totalAmount = source["totalAmount"];
	        this.items = this.convertValues(source["items"], WarehouseSellRecordItem);
	        this.payload = source["payload"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

