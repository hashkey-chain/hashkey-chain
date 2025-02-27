package staking

import (
	"github.com/hashkey-chain/hashkey-chain/common"
	"github.com/hashkey-chain/hashkey-chain/rlp"
	"math/big"
	"testing"
)

var locks = []DelegationLockPeriod{
	{
		1, new(big.Int).SetInt64(10), new(big.Int).SetInt64(10),
	},
	{
		2, new(big.Int).SetInt64(20), new(big.Int).SetInt64(20),
	},
	{
		2, new(big.Int).SetInt64(5), new(big.Int).SetInt64(0),
	},
	{
		3, new(big.Int).SetInt64(0), new(big.Int).SetInt64(30),
	},
}

func newTestDelegationLock() *DelegationLock {
	dlock := NewDelegationLock()

	for _, lock := range locks {
		dlock.AddLock(lock.Epoch, lock.Released, lock.RestrictingPlan)
	}
	return dlock
}

func TestDelegationLock_Add(t *testing.T) {
	dlock := newTestDelegationLock()

	if len(dlock.Locks) != 3 {
		t.Error("locks should be 3")
	}

	if dlock.Locks[0].Released.Cmp(locks[0].Released) != 0 && dlock.Locks[0].RestrictingPlan.Cmp(locks[0].RestrictingPlan) != 0 {
		t.Error("epoch 1 release should be same")
	}

	if dlock.Locks[1].Released.Cmp(new(big.Int).SetInt64(25)) != 0 {
		t.Error("epoch 2 release should be same")
	}

	if dlock.Locks[2].Released.Cmp(new(big.Int).SetInt64(0)) != 0 {
		t.Error("epoch 3 release should be same")
	}

}

func TestDelegationLock_AdvanceLockedFunds(t *testing.T) {
	dlock := newTestDelegationLock()

	released1, restrictingPlan1, err := dlock.AdvanceLockedFunds(big.NewInt(15))
	if err != nil {
		t.Error(err)
	}
	if restrictingPlan1.Cmp(big.NewInt(15)) != 0 || released1.Cmp(common.Big0) != 0 {
		t.Error("release or restrictingPlan seems wrong")
	}
	if len(dlock.Locks) != 3 {
		t.Error("delegationLock length seems wrong")
	}

	released2, restrictingPlan2, err := dlock.AdvanceLockedFunds(big.NewInt(15))
	if err != nil {
		t.Error(err)
	}
	if restrictingPlan2.Cmp(big.NewInt(15)) != 0 || released2.Cmp(common.Big0) != 0 {
		t.Error("release or restrictingPlan seems wrong")
	}
	if len(dlock.Locks) != 2 {
		t.Error("delegationLock length seems wrong")
	}

	released3, restrictingPlan3, err := dlock.AdvanceLockedFunds(big.NewInt(60))
	if err != nil {
		t.Error(err)
	}
	if restrictingPlan3.Cmp(big.NewInt(30)) != 0 || released3.Cmp(big.NewInt(30)) != 0 {
		t.Error("release or restrictingPlan seems wrong")
	}
	if len(dlock.Locks) != 1 {
		t.Error("delegationLock seems wrong")
	}

	_, _, err3 := dlock.AdvanceLockedFunds(big.NewInt(60))
	if err3 != ErrDelegateLockBalanceNotEnough {
		t.Error("should ErrDelegateLockBalanceNotEnough")
	}

	_, _, err4 := dlock.AdvanceLockedFunds(big.NewInt(5))
	if err4 != nil {
		t.Error(err4)
	}
	if len(dlock.Locks) != 0 {
		t.Error("delegationLock seems wrong")
	}
}

func TestDelegationLock_update(t *testing.T) {
	dlock := newTestDelegationLock()
	dlock.update(0)
	if len(dlock.Locks) != 3 {
		t.Error("update wrong")
	}
	dlock = newTestDelegationLock()
	dlock.update(1)
	if len(dlock.Locks) != 3 {
		t.Error("update wrong")
	}
	dlock = newTestDelegationLock()
	dlock.update(2)
	if len(dlock.Locks) != 2 {
		t.Error("update wrong")
	}
	dlock = newTestDelegationLock()
	dlock.update(3)
	if len(dlock.Locks) != 1 {
		t.Error("update wrong")
	}
	dlock = newTestDelegationLock()
	dlock.update(4)
	if len(dlock.Locks) != 0 {
		t.Error("update wrong")
	}
}

func TestDelegationLock_shouldDel(t *testing.T) {
	dlock := newTestDelegationLock()
	dlock.update(2)
	if dlock.shouldDel() {
		t.Error("should not del")
	}

	dlock.update(4)
	dlock.Released = new(big.Int)
	dlock.RestrictingPlan = new(big.Int)

	if !dlock.shouldDel() {
		t.Error("should del")
	}
}

func TestDelegation_rlp(t *testing.T) {
	delegation := NewDelegation()
	delegation.DelegateEpoch = 1
	delegation.Released = new(big.Int).SetInt64(200)
	delegation.LockReleasedHes = new(big.Int).SetInt64(100)

	val0, err0 := encodeStoredDelegateRLP(delegation)
	if err0 != nil {
		t.Error(err0)
	}

	val1, err1 := encodeV1StoredDelegateRLP(delegation)
	if err1 != nil {
		t.Error(err1)
	}

	var m DelegationForStorage

	if err := rlp.DecodeBytes(val0, &m); err != nil {
		t.Error(err)
	}
	if m.LockReleasedHes.Cmp(big.NewInt(100)) != 0 {
		t.Error("decode fail")
	}
	if m.LockRestrictingPlanHes == nil {
		t.Error("decode fail")
	}

	var x DelegationForStorage

	if err := rlp.DecodeBytes(val1, &x); err != nil {
		t.Error(err)
	}
	if x.LockReleasedHes.Cmp(big.NewInt(0)) != 0 {
		t.Error("decode fail")
	}
	if x.LockRestrictingPlanHes == nil {
		t.Error("decode fail")
	}
}
