package ei

// Research data structure
type Research struct {
	id         string
	maxLevel   int
	epic       bool
	shipping   bool
	deliveries bool
}

/*

"epicResearch": [
          {
            "id": "hold_to_hatch",
            "level": 15
          },
          {
            "id": "epic_hatchery",
            "level": 20
          },
          {
            "id": "epic_internal_incubators",
            "level": 20
          },
          {
            "id": "video_doubler_time",
            "level": 1
          },
          {
            "id": "epic_clucking",
            "level": 20
          },
          {
            "id": "epic_multiplier",
            "level": 100
          },
          {
            "id": "cheaper_contractors",
            "level": 10
          },
          {
            "id": "bust_unions",
            "level": 10
          },
          {
            "id": "cheaper_research",
            "level": 10
          },
          {
            "id": "epic_silo_quality",
            "level": 0
          },
          {
            "id": "silo_capacity",
            "level": 20
          },
          {
            "id": "int_hatch_sharing",
            "level": 10
          },
          {
            "id": "int_hatch_calm",
            "level": 20
          },
          {
            "id": "accounting_tricks",
            "level": 20
          },
          {
            "id": "hold_to_research",
            "level": 8
          },
          {
            "id": "soul_eggs",
            "level": 140
          },
          {
            "id": "prestige_bonus",
            "level": 20
          },
          {
            "id": "drone_rewards",
            "level": 20
          },
          {
            "id": "epic_egg_laying",
            "level": 20
          },
          {
            "id": "transportation_lobbyist",
            "level": 30
          },
          {
            "id": "warp_shift",
            "level": 0
          },
          {
            "id": "prophecy_bonus",
            "level": 3
          },
          {
            "id": "afx_mission_time",
            "level": 12
          },
          {
            "id": "afx_mission_capacity",
            "level": 1
          }
        ],



"habs": [
          18,
          18,
          18,
          18
        ],
        "habPopulation": [
          3260250000,
          3260250000,
          3260250000,
          3260250000
        ],
        "vehicles": [
          11,
          11,
          11,
          11,
          11,
          11,
          11,
          11,
          11,
          11,
          11,
          11,
          11,
          11,
          11,
          11,
          11
        ],
        "trainLength": [
          10,
          10,
          10,
          10,
          10,
          10,
          10,
          10,
          10,
          10,
          10,
          10,
          10,
          10,
          10,
          10,
          10
        ],
        "silosOwned": 10,
        "commonResearch": [
          {
            "id": "comfy_nests",
            "level": 50
          },
          {
            "id": "nutritional_sup",
            "level": 40
          },
          {
            "id": "better_incubators",
            "level": 15
          },
          {
            "id": "excitable_chickens",
            "level": 25
          },
          {
            "id": "hab_capacity1",
            "level": 8
          },
          {
            "id": "internal_hatchery1",
            "level": 10
          },
          {
            "id": "padded_packaging",
            "level": 30
          },
          {
            "id": "hatchery_expansion",
            "level": 10
          },
          {
            "id": "bigger_eggs",
            "level": 1
          },
          {
            "id": "internal_hatchery2",
            "level": 10
          },
          {
            "id": "leafsprings",
            "level": 30
          },
          {
            "id": "vehicle_reliablity",
            "level": 2
          },
          {
            "id": "rooster_booster",
            "level": 25
          },
          {
            "id": "coordinated_clucking",
            "level": 50
          },
          {
            "id": "hatchery_rebuild1",
            "level": 1
          },
          {
            "id": "usde_prime",
            "level": 1
          },
          {
            "id": "hen_house_ac",
            "level": 50
          },
          {
            "id": "superfeed",
            "level": 35
          },
          {
            "id": "microlux",
            "level": 10
          },
          {
            "id": "compact_incubators",
            "level": 10
          },
          {
            "id": "lightweight_boxes",
            "level": 40
          },
          {
            "id": "excoskeletons",
            "level": 2
          },
          {
            "id": "internal_hatchery3",
            "level": 15
          },
          {
            "id": "improved_genetics",
            "level": 30
          },
          {
            "id": "traffic_management",
            "level": 2
          },
          {
            "id": "motivational_clucking",
            "level": 50
          },
          {
            "id": "driver_training",
            "level": 30
          },
          {
            "id": "shell_fortification",
            "level": 60
          },
          {
            "id": "egg_loading_bots",
            "level": 2
          },
          {
            "id": "super_alloy",
            "level": 50
          },
          {
            "id": "even_bigger_eggs",
            "level": 5
          },
          {
            "id": "internal_hatchery4",
            "level": 30
          },
          {
            "id": "quantum_storage",
            "level": 20
          },
          {
            "id": "genetic_purification",
            "level": 100
          },
          {
            "id": "internal_hatchery5",
            "level": 250
          },
          {
            "id": "time_compress",
            "level": 20
          },
          {
            "id": "hover_upgrades",
            "level": 25
          },
          {
            "id": "graviton_coating",
            "level": 7
          },
          {
            "id": "grav_plating",
            "level": 25
          },
          {
            "id": "chrystal_shells",
            "level": 100
          },
          {
            "id": "autonomous_vehicles",
            "level": 5
          },
          {
            "id": "neural_linking",
            "level": 21
          },
          {
            "id": "telepathic_will",
            "level": 50
          },
          {
            "id": "enlightened_chickens",
            "level": 150
          },
          {
            "id": "dark_containment",
            "level": 25
          },
          {
            "id": "atomic_purification",
            "level": 50
          },
          {
            "id": "multi_layering",
            "level": 3
          },
          {
            "id": "timeline_diversion",
            "level": 50
          },
          {
            "id": "wormhole_dampening",
            "level": 25
          },
          {
            "id": "eggsistor",
            "level": 100
          },
          {
            "id": "micro_coupling",
            "level": 5
          },
          {
            "id": "neural_net_refine",
            "level": 25
          },
          {
            "id": "matter_reconfig",
            "level": 500
          },
          {
            "id": "timeline_splicing",
            "level": 1
          },
          {
            "id": "hyper_portalling",
            "level": 25
          },
          {
            "id": "relativity_optimization",
            "level": 10
          }
        ],

		"habCapacity": [
          3260250000,
          3260250000,
          3260250000,
          3260250000
        ],



*/
