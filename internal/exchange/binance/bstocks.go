package binance

// bStocksSnapshot lists the base assets Binance tagged "bStocks" (tokenized
// stocks and ETFs) on 2026-10-18. AssetTags falls back to it when the
// product listing cannot be fetched, so a universe that excludes them stays
// closed to them. New listings after that date are only known online.
var bStocksSnapshot = []string{
	"AAOIB", "AAPLB", "ADBEB", "AGPUB", "ALABB", "AMATB", "AMCB", "AMDB", "AMZNB", "ARMB", "ASMLB", "ASTSB",
	"AVGOB", "AXTIB", "BABAB", "BBB", "BEB", "BMNRB", "BNCB", "CBRSB", "COHRB", "COINB", "CRCLB", "CRDOB",
	"CRMB", "CRWDB", "CRWVB", "CYPHB", "DELLB", "DJTB", "DRAMB", "EWYB", "FLNCB", "FWDIB", "GLWB", "GMEB",
	"GOOGLB", "GPROB", "GSB", "HIMSB", "HOODB", "HPEB", "IBMB", "INTCB", "INTWB", "IRENB", "JPMB", "KORUB",
	"LITEB", "LLYB", "MDBB", "METAB", "MRNAB", "MRVLB", "MSFTB", "MSTRB", "MUB", "MUUB", "MVLLB", "NBISB",
	"NFLXB", "NOKB", "NVDAB", "ORCLB", "PDDB", "PENGB", "PLTRB", "PYPLB", "QCOMB", "QNTB", "QQQB", "RDDTB",
	"RKLBB", "SECZB", "SHAZB", "SKHYB", "SMCIB", "SMHB", "SNDKB", "SNXXB", "SOXLB", "SOXSB", "SPCXB", "SPYB",
	"SQQQB", "STXB", "TQQQB", "TSLAB", "TSMB", "USARB", "USDEB", "UVXYB", "WDCB", "WENB", "ZMB",
}
