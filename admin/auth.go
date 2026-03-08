package main

import (
	"net/http"

	"github.com/peterrk/simple-abtest/utils"
)

type privilegeChecker func(uid uint32) (bool, error)

func requireSession(logger *utils.ContextLogger, w http.ResponseWriter, r *http.Request) (uint32, bool) {
	uid, ok := verifySession(logger, w, r)
	if !ok {
		return 0, false
	}
	return uid, true
}

func requireSelf(
	logger *utils.ContextLogger, w http.ResponseWriter, r *http.Request, target uint32,
) (uint32, bool) {
	uid, ok := requireSession(logger, w, r)
	if !ok {
		return 0, false
	}
	if uid != target {
		w.WriteHeader(http.StatusForbidden)
		return 0, false
	}
	return uid, true
}

func requirePrivilege(
	logger *utils.ContextLogger, w http.ResponseWriter, r *http.Request,
	checker privilegeChecker,
) (uint32, bool) {
	uid, ok := requireSession(logger, w, r)
	if !ok {
		return 0, false
	}

	allowed, err := checker(uid)
	if err != nil {
		logger.Errorf("fail to check privilege: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return 0, false
	}
	if !allowed {
		w.WriteHeader(http.StatusForbidden)
		return 0, false
	}
	return uid, true
}

func requireAppPrivilege(
	logger *utils.ContextLogger, w http.ResponseWriter, r *http.Request,
	appId uint32, expected privilegeLevel,
) (uint32, bool) {
	return requirePrivilege(logger, w, r, func(uid uint32) (bool, error) {
		return checkAppPrivilege(uid, appId, expected)
	})
}

func requireExpPrivilege(
	logger *utils.ContextLogger, w http.ResponseWriter, r *http.Request,
	expId uint32, expected privilegeLevel,
) (uint32, bool) {
	return requirePrivilege(logger, w, r, func(uid uint32) (bool, error) {
		return checkExpPrivilege(uid, expId, expected)
	})
}

func requireLyrPrivilege(
	logger *utils.ContextLogger, w http.ResponseWriter, r *http.Request,
	lyrId uint32, expected privilegeLevel,
) (uint32, bool) {
	return requirePrivilege(logger, w, r, func(uid uint32) (bool, error) {
		return checkLyrPrivilege(uid, lyrId, expected)
	})
}

func requireSegPrivilege(
	logger *utils.ContextLogger, w http.ResponseWriter, r *http.Request,
	segId uint32, expected privilegeLevel,
) (uint32, bool) {
	return requirePrivilege(logger, w, r, func(uid uint32) (bool, error) {
		return checkSegPrivilege(uid, segId, expected)
	})
}

func requireGrpPrivilege(
	logger *utils.ContextLogger, w http.ResponseWriter, r *http.Request,
	grpId uint32, expected privilegeLevel,
) (uint32, bool) {
	return requirePrivilege(logger, w, r, func(uid uint32) (bool, error) {
		return checkGrpPrivilege(uid, grpId, expected)
	})
}
