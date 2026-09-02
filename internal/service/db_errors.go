package service

import "github.com/truewhile/MeBox/internal/repository"

func IsTransientDatabaseLock(err error) bool {
	return repository.IsSQLiteBusyError(err)
}
