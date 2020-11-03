package controllers

type ReconciliationAction string

const (
	reconcileAdd    ReconciliationAction = "Add"
	reconcileUpdate ReconciliationAction = "Update"
	reconcileDelete ReconciliationAction = "Delete"
	reconcilePoll   ReconciliationAction = "Poll"
)
