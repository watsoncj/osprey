package flagging

func defaultCategories() map[string][]string {
	return map[string][]string{
		"adult_content": {
			"pornhub", "xvideos", "xnxx", "xhamster", "redtube",
			"youporn", "brazzers", "onlyfans", "chaturbate",
			"pornographic", "nsfw", "xxx ", "playboy",
		},
		"violence": {
			"bestgore", "liveleak", "theync",
			"graphic violence", "execution video", "snuff",
			"school shooting", "mass shooting", "sniper", "assault rifle",
		},
		"bullying": {
			"cyberbullying", "kill yourself",
			"you should die", "nobody likes you",
			"go die", "end your life",
		},
		"self_harm": {
			"self-harm", "self harm", "cutting myself",
			"suicide method", "how to kill myself",
			"want to die", "suicidal",
		},
		"drugs": {
			"buy drugs online", "silk road market",
			"how to make meth", "drug dealer",
			"darknet market", "buy cocaine",
			"buy fentanyl",
		},
		"weapons": {
			"buy gun illegally", "ghost gun",
			"3d print gun", "how to make a bomb",
			"build explosive",
		},
		"hate_speech": {
			"white supremacy", "white power",
			"neo-nazi", "race war",
			"ethnic cleansing",
		},
	}
}
