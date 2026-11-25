package commerce

// Plugins are assembled once, before serving requests.
type Plugins struct {
	Catalog   CatalogProvider
	Effects   EffectRegistry
	Providers ProviderRegistry
}

func DefaultPlugins() Plugins {
	voucher := &Voucher{codes: map[string]int64{"MAKU2026": 600}}
	return Plugins{DefaultCatalog(), DefaultEffects(), ProviderRegistry{"sandbox": NewSandbox(""), "voucher": voucher}}
}
func LoadPlugins(path, secret, codes string) (Plugins, error) {
	p := DefaultPlugins()
	if path != "" {
		catalog, err := LoadCatalog(path, p.Effects)
		if err != nil {
			return Plugins{}, err
		}
		p.Catalog = catalog
	}
	if secret != "" {
		p.Providers["sandbox"] = NewSandbox(secret)
	}
	if codes != "" {
		v, err := NewVoucher(codes)
		if err != nil {
			return Plugins{}, err
		}
		p.Providers["voucher"] = v
	}
	return p, nil
}
