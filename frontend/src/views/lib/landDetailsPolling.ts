export type LandRecord = {
  id: string;
  landId: number;
};

export type LandDetailsSnapshot<T extends LandRecord = LandRecord> = {
  revision?: string;
  lands: T[];
  [key: string]: unknown;
};

export type LandDetailsDelta<T extends LandRecord = LandRecord> = {
  full: boolean;
  revision: string;
  lands: T[];
  removedLandIds: number[];
  [key: string]: unknown;
};

export function mergeLandDetailsDelta<T extends LandRecord>(
  current: LandDetailsSnapshot<T> | undefined,
  delta: LandDetailsDelta<T>,
): LandDetailsSnapshot<T> {
  const { full, lands, removedLandIds, ...fields } = delta;
  const definedFields = Object.fromEntries(Object.entries(fields).filter(([, value]) => value !== undefined));
  if (full || !current) {
    return { ...definedFields, lands } as LandDetailsSnapshot<T>;
  }

  const updates = new Map(lands.map((land) => [land.landId, land]));
  const removed = new Set(removedLandIds);
  const merged = current.lands
    .filter((land) => !removed.has(land.landId))
    .map((land) => updates.get(land.landId) || land);
  for (const land of lands) {
    if (!current.lands.some((currentLand) => currentLand.landId === land.landId)) {
      merged.push(land);
    }
  }
  return { ...current, ...definedFields, lands: merged };
}
