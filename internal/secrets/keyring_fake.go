package secrets

// FakeStore 是内存版 SecretStore，供测试注入（避免触碰真实 Keychain）。
type FakeStore struct {
	data map[string]string
}

// NewFake 返回空的 FakeStore。
func NewFake() *FakeStore { return &FakeStore{data: map[string]string{}} }

func (f *FakeStore) Get(service string) (string, error) {
	if v, ok := f.data[service]; ok {
		return v, nil
	}
	return "", ErrNotFound
}

func (f *FakeStore) Set(service, value string) error {
	f.data[service] = value
	return nil
}

func (f *FakeStore) Delete(service string) error {
	delete(f.data, service)
	return nil // 幂等
}
