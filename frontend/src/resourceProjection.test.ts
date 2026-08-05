import { describe, expect, test } from "bun:test";
import { projectStoredResources, type StoredResources } from "./resourceProjection";

const resources: StoredResources = {
  metal: 1_000,
  crystal: 2_000,
  deuterium: 3_000,
  metalCapacity: 10_000,
  crystalCapacity: 10_000,
  deuteriumCapacity: 10_000,
  productionPerHour: {
    metal: 3_600,
    crystal: 1_800,
    deuterium: -360
  }
};

describe("client-side resource projection", () => {
  test("projects the server snapshot with hourly production", () => {
    expect(projectStoredResources(resources, 30 * 60 * 1000)).toEqual({
      metal: 2_800,
      crystal: 2_900,
      deuterium: 2_820
    });
  });

  test("matches legacy storage caps", () => {
    expect(
      projectStoredResources(
        {
          ...resources,
          metal: 9_500,
          crystal: 12_000,
          productionPerHour: { metal: 3_600, crystal: -3_600, deuterium: 0 }
        },
        60 * 60 * 1000
      )
    ).toEqual({
      metal: 10_000,
      crystal: 12_000,
      deuterium: 3_000
    });
  });

  test("does not project backwards or without production metadata", () => {
    expect(projectStoredResources({ ...resources, productionPerHour: undefined }, -1_000)).toEqual({
      metal: 1_000,
      crystal: 2_000,
      deuterium: 3_000
    });
  });
});
