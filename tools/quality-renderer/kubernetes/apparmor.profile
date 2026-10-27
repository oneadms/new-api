# new-api / QuantumNous — renderer-only AppArmor confinement.
# Derived from containerd v2.2.7 contrib/apparmor/template.go (Apache-2.0).
# Copyright The docker Authors.
# Copyright The Moby Authors.
# Copyright The containerd Authors.
# Licensed under the Apache License, Version 2.0; see LICENSE.apache-2.0.
# http://www.apache.org/licenses/LICENSE-2.0
# Distributed on an AS IS BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND.
# See the License for the specific language governing permissions and limitations.

abi <abi/3.0>,
#include <tunables/global>

profile new-api-quality-renderer flags=(attach_disconnected,mediate_deleted) {
  #include <abstractions/base>
  network,
  capability,
  file,
  umount,
  signal (receive) peer=unconfined,
  signal (receive) peer=runc,
  signal (receive) peer=crun,
  signal (send,receive) peer=new-api-quality-renderer,

  deny @{PROC}/* w,
  deny @{PROC}/{[^1-9],[^1-9][^0-9],[^1-9s][^0-9y][^0-9s],[^1-9][^0-9][^0-9][^0-9]*}/** w,
  deny @{PROC}/sys/[^k]** w,
  deny @{PROC}/sys/kernel/{?,??,[^s][^h][^m]**} w,
  deny @{PROC}/sysrq-trigger rwklx,
  deny @{PROC}/mem rwklx,
  deny @{PROC}/kmem rwklx,
  deny @{PROC}/kcore rwklx,
  deny mount,
  deny /sys/[^f]*/** wklx,
  deny /sys/f[^s]*/** wklx,
  deny /sys/fs/[^c]*/** wklx,
  deny /sys/fs/c[^g]*/** wklx,
  deny /sys/fs/cg[^r]*/** wklx,
  deny /sys/firmware/** rwklx,
  deny /sys/devices/virtual/powercap/** rwklx,
  deny /sys/kernel/security/** rwklx,
  ptrace (trace,tracedby,read,readby) peer=new-api-quality-renderer,

  # Kubernetes emptyDir does not offer a per-volume noexec option. Deny both
  # execution and executable file mappings on all renderer-writable paths.
  deny /tmp/** mx,
  deny /dev/shm/** mx,
  deny /run/quality-renderer/** mx,
}
