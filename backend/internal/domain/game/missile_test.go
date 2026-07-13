package game

import "testing"

func TestResolveMissileAttackMatchesLegacyCases(t *testing.T) {
	for _, test := range []struct {
		name        string
		input       MissileAttackInput
		intercepted int
		want        DefenseCounts
	}{
		{
			name:        "full interception",
			input:       MissileAttackInput{Amount: 3, PrimaryDefenseID: DefenseRocketLauncher, Target: DefenseCounts{DefenseAntiBallisticMissile: 5, DefenseRocketLauncher: 10, DefenseLightLaser: 5}},
			intercepted: 3, want: DefenseCounts{DefenseAntiBallisticMissile: 2, DefenseRocketLauncher: 10, DefenseLightLaser: 5},
		},
		{
			name:        "partial targeted",
			input:       MissileAttackInput{Amount: 3, PrimaryDefenseID: DefenseRocketLauncher, Target: DefenseCounts{DefenseAntiBallisticMissile: 2, DefenseRocketLauncher: 100, DefenseLightLaser: 10}},
			intercepted: 2, want: DefenseCounts{DefenseAntiBallisticMissile: 0, DefenseRocketLauncher: 40, DefenseLightLaser: 10},
		},
		{
			name:  "targeted plasma",
			input: MissileAttackInput{Amount: 1, PrimaryDefenseID: DefensePlasmaTurret, Target: DefenseCounts{DefensePlasmaTurret: 3}},
			want:  DefenseCounts{DefensePlasmaTurret: 2},
		},
		{
			name:  "ordered sweep",
			input: MissileAttackInput{Amount: 1, Target: DefenseCounts{DefenseRocketLauncher: 20, DefenseLightLaser: 20}},
			want:  DefenseCounts{DefenseRocketLauncher: 0, DefenseLightLaser: 0},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := ResolveMissileAttack(test.input)
			if result.Intercepted != test.intercepted {
				t.Fatalf("intercepted=%d, want %d", result.Intercepted, test.intercepted)
			}
			for id, want := range test.want {
				if result.Target[id] != want {
					t.Fatalf("defense %d=%d, want %d; result=%+v", id, result.Target[id], want, result.Target)
				}
			}
		})
	}
}

func TestResolveMissileAttackUsesPlanetInterceptorsForMoon(t *testing.T) {
	input := MissileAttackInput{
		Amount: 2, MoonAttack: true, PrimaryDefenseID: DefenseRocketLauncher,
		Target:          DefenseCounts{DefenseRocketLauncher: 20},
		DefendingPlanet: DefenseCounts{DefenseAntiBallisticMissile: 1},
	}
	result := ResolveMissileAttack(input)
	if result.Intercepted != 1 || result.DefendingPlanet[DefenseAntiBallisticMissile] != 0 || result.Target[DefenseRocketLauncher] != 0 {
		t.Fatalf("unexpected moon missile result: %+v", result)
	}
	if input.DefendingPlanet[DefenseAntiBallisticMissile] != 1 || input.Target[DefenseRocketLauncher] != 20 {
		t.Fatal("resolver must not mutate input maps")
	}
}
