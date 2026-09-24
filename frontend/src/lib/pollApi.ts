import type { Tab } from '../components/AppShell';
import type { FarmAutomationState } from '../views/AutomationView';
import type { HostBindingStatusDto } from '../views/GuardView';
import type { RuntimeStatusDto, WorkspaceRunStatistics } from '../views/OverviewView';
import type { DogGuardState } from '../views/SocialView';
import type { RuntimeEventDto } from './events';
import type { QQPatchStatusLike } from './startupInjection';

export type DashboardPollResponse = {
  status: RuntimeStatusDto;
  guardStatus: Record<string, unknown>;
  bindingStatus: HostBindingStatusDto;
  events: RuntimeEventDto[];
  tsdkEvents: RuntimeEventDto[];
  automationState: FarmAutomationState;
  runStatistics?: WorkspaceRunStatistics;
  patchStatus: QQPatchStatusLike;
};

export type PollLandRecord = Record<string, unknown> & { id: string; landId: number };

export type LandPollResponse = {
  full: boolean;
  revision: string;
  status?: string;
  message?: string;
  farmType?: string;
  totalGrids?: number;
  lands: PollLandRecord[];
  removedLandIds: number[];
  actions?: Array<Record<string, unknown>>;
  runtimeError?: string;
};

export class PollApiError extends Error {
  constructor(message: string, readonly status: number) {
    super(message);
    this.name = 'PollApiError';
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

async function readPoll(path: string, signal: AbortSignal): Promise<unknown> {
  const response = await fetch(path, { cache: 'no-store', signal });
  if (!response.ok) {
    throw new PollApiError(`Poll request failed with HTTP ${response.status}`, response.status);
  }
  return response.json();
}

export async function pollDashboard(activeTab: Tab, signal: AbortSignal): Promise<DashboardPollResponse> {
  const value = await readPoll(`/farm-api/poll/dashboard?${new URLSearchParams({ activeTab })}`, signal);
  if (!isRecord(value) || !isRecord(value.status) || !isRecord(value.guardStatus) ||
      !isRecord(value.bindingStatus) || !Array.isArray(value.events) || !Array.isArray(value.tsdkEvents) ||
      !isRecord(value.automationState) || !isRecord(value.patchStatus)) {
    throw new Error('Invalid dashboard poll response');
  }
  return value as DashboardPollResponse;
}

export async function pollLand(revision: string, signal: AbortSignal): Promise<LandPollResponse> {
  const value = await readPoll(`/farm-api/poll/land?${new URLSearchParams({ revision })}`, signal);
  if (!isRecord(value) || typeof value.full !== 'boolean' || typeof value.revision !== 'string' ||
      !Array.isArray(value.lands) || !Array.isArray(value.removedLandIds)) {
    throw new Error('Invalid land poll response');
  }
  return value as LandPollResponse;
}

export async function pollDogGuard(signal: AbortSignal): Promise<DogGuardState> {
  const value = await readPoll('/farm-api/poll/dog-guard', signal);
  if (!isRecord(value) || !Array.isArray(value.results)) {
    throw new Error('Invalid dog guard poll response');
  }
  return value as DogGuardState;
}
