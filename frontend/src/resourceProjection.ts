export type ResourceProductionPerHour = {
  metal: number;
  crystal: number;
  deuterium: number;
};

export type StoredResources = {
  metal: number;
  crystal: number;
  deuterium: number;
  metalCapacity: number;
  crystalCapacity: number;
  deuteriumCapacity: number;
  productionPerHour?: ResourceProductionPerHour;
};

export type ProjectedStoredResources = Pick<StoredResources, "metal" | "crystal" | "deuterium">;

export function projectStoredResources(resources: StoredResources, elapsedMilliseconds: number): ProjectedStoredResources {
  const elapsedSeconds = Math.max(0, elapsedMilliseconds) / 1000;
  const production = resources.productionPerHour;
  return {
    metal: projectStoredResource(resources.metal, production?.metal ?? 0, resources.metalCapacity, elapsedSeconds),
    crystal: projectStoredResource(resources.crystal, production?.crystal ?? 0, resources.crystalCapacity, elapsedSeconds),
    deuterium: projectStoredResource(
      resources.deuterium,
      production?.deuterium ?? 0,
      resources.deuteriumCapacity,
      elapsedSeconds
    )
  };
}

function projectStoredResource(current: number, hourly: number, capacity: number, elapsedSeconds: number): number {
  if (elapsedSeconds <= 0 || current >= capacity) {
    return current;
  }
  return Math.min(current + (hourly * elapsedSeconds) / 3600, capacity);
}
