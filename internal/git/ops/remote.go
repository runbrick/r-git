package ops

import "context"

func (rm *Service) AddRemote(ctx context.Context, name, url string) error {
	_, err := rm.run(ctx, "remote", "add", name, url)
	return err
}

func (rm *Service) RemoveRemote(ctx context.Context, name string) error {
	_, err := rm.run(ctx, "remote", "remove", name)
	return err
}

func (rm *Service) RenameRemote(ctx context.Context, oldName, newName string) error {
	_, err := rm.run(ctx, "remote", "rename", oldName, newName)
	return err
}

func (rm *Service) SetRemoteURL(ctx context.Context, name, url string) error {
	_, err := rm.run(ctx, "remote", "set-url", name, url)
	return err
}
