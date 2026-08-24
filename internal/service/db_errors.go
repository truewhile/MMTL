package service

import "github.com/ShukeBta/MMTL/internal/repository"

func IsTransientDatabaseLock(err error) bool {
	return repository.IsSQLiteBusyError(err)
}
