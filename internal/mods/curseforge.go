package mods

import (
	"strings"
	"vesper-launcher/internal/models"
)

var CurseForgePreviewCatalog = []models.ModItem{
	{
		ProjectId:    "cf-jei",
		DisplayName:  "Just Enough Items (JEI)",
		Description:  "Позволяет просматривать рецепты крафта и все доступные предметы прямо в инвентаре.",
		Author:       "mezz",
		IconUrl:      "https://cdn.modrinth.com/data/u6dRKJwZ/4a3f18ac0d096c9f8e9176984c44be4e58f94c89_96.webp",
		Downloads:    248500000,
		Followers:    154000,
		ContentKind:  "mod",
		Categories:   []string{"Инвентарь", "Удобство"},
		GameVersions: []string{"1.20.1", "1.19.2", "1.18.2", "1.16.5", "1.12.2"},
		Loaders:      []string{"Forge", "Fabric", "NeoForge"},
		CanInstall:   true,
		ActionText:   "Установить",
	},
	{
		ProjectId:    "cf-journeymap",
		DisplayName:  "JourneyMap",
		Description:  "Отображает карту мира в реальном времени, миникарту на экране и позволяет ставить метки.",
		Author:       "techbrew",
		IconUrl:      "https://cdn.modrinth.com/data/lfHFW1mp/a1c571a21a88f6fa59eab67829f216f65ab393ee_96.webp",
		Downloads:    142300000,
		Followers:    92000,
		ContentKind:  "mod",
		Categories:   []string{"Навигация", "Мини-карта"},
		GameVersions: []string{"1.20.1", "1.19.2", "1.18.2", "1.16.5"},
		Loaders:      []string{"Forge", "Fabric", "NeoForge"},
		CanInstall:   true,
		ActionText:   "Установить",
	},
	{
		ProjectId:    "cf-bop",
		DisplayName:  "Biomes O' Plenty",
		Description:  "Добавляет огромное количество уникальных биомов с новыми деревьями, цветами и блоками.",
		Author:       "Forstride",
		IconUrl:      "https://cdn.modrinth.com/data/HXF82T3G/ffb870e12c325b795d54833f8f899126553ef06f.png",
		Downloads:    118700000,
		Followers:    78000,
		ContentKind:  "mod",
		Categories:   []string{"Мир", "Генерация"},
		GameVersions: []string{"1.20.1", "1.19.2", "1.18.2", "1.16.5"},
		Loaders:      []string{"Forge", "NeoForge"},
		CanInstall:   true,
		ActionText:   "Установить",
	},
	{
		ProjectId:    "cf-tweaks",
		DisplayName:  "Mouse Tweaks",
		Description:  "Облегчает сортировку инвентаря и перемещение предметов с помощью зажатой кнопки мыши.",
		Author:       "YaLTeR",
		IconUrl:      "https://cdn.modrinth.com/data/aC3cM3Vq/6c0eaa4e60a9c87f4766f222ff63286f09da32c0_96.webp",
		Downloads:    128400000,
		Followers:    68000,
		ContentKind:  "mod",
		Categories:   []string{"Удобство", "Инвентарь"},
		GameVersions: []string{"1.20.1", "1.19.2", "1.18.2", "1.16.5", "1.12.2"},
		Loaders:      []string{"Forge", "Fabric", "NeoForge"},
		CanInstall:   true,
		ActionText:   "Установить",
	},
	{
		ProjectId:    "cf-faithful",
		DisplayName:  "Faithful 32x",
		Description:  "Оригинальные текстуры игры в более высоком и детализированном качестве.",
		Author:       "FaithfulTeam",
		IconUrl:      "https://cdn.modrinth.com/data/w0TnApzs/e8403d1fb2f55321ae74402c1e8c90a3a5670856.png",
		Downloads:    85400000,
		Followers:    42000,
		ContentKind:  "resourcepack",
		Categories:   []string{"Ресурспаки", "32x"},
		GameVersions: []string{"1.20.1", "1.19.2", "1.18.2", "1.16.5"},
		CanInstall:   true,
		ActionText:   "Установить",
	},
	{
		ProjectId:    "cf-bsl",
		DisplayName:  "BSL Shaders",
		Description:  "Популярные шейдеры с мягким реалистичным освещением, туманом и красивой водой.",
		Author:       "CaptTatsu",
		IconUrl:      "https://cdn.modrinth.com/data/Q1vvjJYV/2a611a3cb434fb52fb81fa5dace13c5d8b67e55d_96.webp",
		Downloads:    38200000,
		Followers:    29000,
		ContentKind:  "shader",
		Categories:   []string{"Шейдеры", "Реализм"},
		GameVersions: []string{"1.20.1", "1.19.2", "1.18.2", "1.16.5"},
		CanInstall:   true,
		ActionText:   "Установить",
	},
	{
		ProjectId:    "cf-complimentary",
		DisplayName:  "Complementary Shaders",
		Description:  "Идеальный баланс производительности и красоты, сохраняющий дух ванили.",
		Author:       "EminGT",
		IconUrl:      "https://cdn.modrinth.com/data/HVnmMxH1/79cb7c8123bbc54945305b2ebad6b8881efdf5f8_96.webp",
		Downloads:    52700000,
		Followers:    38000,
		ContentKind:  "shader",
		Categories:   []string{"Шейдеры", "Оптимизация"},
		GameVersions: []string{"1.20.1", "1.19.2", "1.18.2", "1.16.5"},
		CanInstall:   true,
		ActionText:   "Установить",
	},
	{
		ProjectId:    "cf-rlcraft",
		DisplayName:  "RLCraft",
		Description:  "Сверхсложная сборка на выживание с драконами, фэнтези и реалистичной физикой.",
		Author:       "Shivaxi",
		IconUrl:      "https://cdn.modrinth.com/data/Qx4KOI2G/6bce4b7f4a25a49e23d57fcc6838a1c46b0aff72_96.webp",
		Downloads:    18500000,
		Followers:    19000,
		ContentKind:  "modpack",
		Categories:   []string{"Сборки", "Выживание"},
		GameVersions: []string{"1.12.2"},
		CanInstall:   true,
		ActionText:   "Установить",
	},
}

func SearchCurseForge(query, category string) []models.ModItem {
	targetKind := "mod"
	switch category {
	case "Ресурспаки", "Ресурс паки", "resourcepack":
		targetKind = "resourcepack"
	case "Сборки", "modpack":
		targetKind = "modpack"
	case "Шейдеры", "shader":
		targetKind = "shader"
	default:
		targetKind = "mod"
	}

	q := strings.ToLower(strings.TrimSpace(query))
	var result []models.ModItem
	for _, it := range CurseForgePreviewCatalog {
		if it.ContentKind != targetKind {
			continue
		}
		if q == "" || strings.Contains(strings.ToLower(it.DisplayName), q) || strings.Contains(strings.ToLower(it.Description), q) {
			result = append(result, it)
		}
	}
	return result
}
