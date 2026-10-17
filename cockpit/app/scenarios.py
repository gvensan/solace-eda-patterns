"""Scenario registry.

A scenario is a folder under cockpit/scenarios/<id>/ containing a
scenario.yaml descriptor plus whatever it needs to run (terraform config,
sample app source, and so on). Adding a workshop module means dropping in a
folder -- no cockpit code changes.
"""

import logging
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional

import yaml

from .config import SCENARIOS_DIR, scenario_state_dir, terraform_env

log = logging.getLogger("uvicorn.error")


@dataclass
class Action:
    """One button on a scenario card."""

    id: str
    label: str
    kind: str  # "terraform" | "process"
    description: str = ""
    # terraform actions
    command: str = "apply"  # apply | destroy | plan
    tf_dir: str = "tf"
    # process actions
    cmd: list = field(default_factory=list)
    cwd: Optional[str] = None
    long_running: bool = False
    # presentation
    variant: str = "primary"  # primary | secondary | danger
    confirm: Optional[str] = None
    # Exists only to cause a failure mode, so it is left out of the Actions
    # flowchart and reached from that failure mode's card instead.
    failure_only: bool = False

    @property
    def is_destructive(self) -> bool:
        return self.variant == "danger" or self.command == "destroy"


@dataclass
class Inspect:
    """A read-only view rendered after a scenario runs, so attendees can see
    what the configuration actually produced before touching the broker UI."""

    label: str
    kind: str  # "semp"
    path: str
    columns: list = field(default_factory=list)
    ui_hint: str = ""
    # "config" for objects, "monitor" for live counters. See broker.semp_get.
    api: str = "config"
    # What an empty table means for this view, and what fills it. A queue is
    # empty until configuration is applied; its consumers until apps run.
    empty: str = ""


# Broker object kinds a scenario can own at the top level, mapped to their SEMP
# collection. Listed in the order they have to be deleted: a client username
# refers to its profiles, so it goes before them.
_TOP_LEVEL = {
    "client_username": "clientUsernames",
    "queue": "queues",
    "acl_profile": "aclProfiles",
    "client_profile": "clientProfiles",
}


@dataclass
class Scenario:
    id: str
    title: str
    summary: str
    eyebrow: str
    root: Path
    order: int = 100
    objectives: list = field(default_factory=list)
    actions: list = field(default_factory=list)
    inspect: list = field(default_factory=list)
    # Optional node/flow diagram animated at the top of the scenario page.
    # Passed through to the browser as-is; the cockpit renders it.
    diagram: dict = field(default_factory=dict)
    # Ordered action ids for the one-click "Run scenario" button. Declared
    # per scenario because the right sequence is a workshop decision, not
    # something that can be inferred: consumers must be up before anything
    # is published (exclusive-consumer also binds ledger A before ledger B,
    # which decides the active one), and no scenario wants its destroy action
    # included.
    run: list = field(default_factory=list)
    # Ordered action ids for Cleanup: tears down whatever Play created. Usually
    # a terraform destroy, but a scenario may need several steps.
    cleanup: list = field(default_factory=list)
    # Ways to break the running scenario on purpose, each with the steps that
    # cause it and the steps that put things back. Passed to the browser
    # as-is; the cockpit drives them with the ordinary start and stop calls.
    failure_modes: list = field(default_factory=list)
    # What smoke.py checks once Play has finished (`play:`), in the same
    # form as a failure mode's expect: block.
    expect: dict = field(default_factory=dict)
    # Choices an attendee can make about how an app runs (how fast a
    # publisher goes, say). Each choice is a set of arguments, put into any
    # action's cmd where it says "{setting:<name>}".
    settings: dict = field(default_factory=dict)

    @property
    def run_sequence(self) -> list:
        """The actions Run scenario executes, in order.

        Falls back to every non-destructive action in declaration order, which
        is right for a simple scenario and wrong often enough that anything
        with ordering constraints should declare `run:` explicitly.
        """
        if self.run:
            resolved = [self.action(aid) for aid in self.run]
            return [a for a in resolved if a is not None]
        return [a for a in self.actions if not a.is_destructive]

    @property
    def cleanup_sequence(self) -> list:
        """The actions Cleanup executes, in order.

        Defaults to every destructive action, which is the destroy step in a
        typical scenario. Declared explicitly when the teardown needs more.
        """
        if self.cleanup:
            resolved = [self.action(aid) for aid in self.cleanup]
            return [a for a in resolved if a is not None]
        return [a for a in self.actions if a.is_destructive]

    def owned_objects(self) -> list:
        """(kind, name) for every top-level broker object this scenario owns,
        read from tf/imports.tsv, the same list Reconcile uses. Children such
        as queue subscriptions and ACL exceptions are left out: they go when
        their parent is deleted."""
        imports = self.root / "tf" / "imports.tsv"
        if not imports.exists():
            return []
        found = []
        for line in imports.read_text().splitlines():
            if not line.strip() or line.startswith("#") or "\t" not in line:
                continue
            address, import_id = line.split("\t", 1)
            kind = address.split(".", 1)[0].removeprefix("solacebroker_msg_vpn_")
            if kind in _TOP_LEVEL:
                found.append((kind, import_id.strip().split("/", 1)[1]))
        return found

    def action(self, action_id: str) -> Optional[Action]:
        return next((a for a in self.actions if a.id == action_id), None)

    def tf_workdir(self, action: Action) -> Path:
        return self.root / action.tf_dir

    def as_dict(self) -> dict:
        return {
            "id": self.id,
            "title": self.title,
            "summary": self.summary,
            "eyebrow": self.eyebrow,
            "order": self.order,
            "objectives": self.objectives,
            "actions": [
                {
                    "id": a.id,
                    "label": a.label,
                    "description": a.description,
                    "kind": a.kind,
                    "variant": a.variant,
                    "confirm": a.confirm,
                    "longRunning": a.long_running,
                    "failureOnly": a.failure_only,
                }
                for a in self.actions
            ],
            "inspect": [
                {"label": i.label, "kind": i.kind, "path": i.path,
                 "columns": i.columns, "uiHint": i.ui_hint, "empty": i.empty}
                for i in self.inspect
            ],
            "runSequence": [a.id for a in self.run_sequence],
            "cleanupSequence": [a.id for a in self.cleanup_sequence],
            "diagram": self.diagram,
            "failureModes": self.failure_modes,
            "expect": self.expect,
            "settings": self.settings,
        }


def _load_one(path: Path) -> Scenario:
    raw = yaml.safe_load(path.read_text()) or {}
    root = path.parent
    return Scenario(
        id=raw.get("id", root.name),
        title=raw.get("title", root.name),
        summary=raw.get("summary", ""),
        eyebrow=raw.get("eyebrow", "Scenario"),
        order=raw.get("order", 100),
        objectives=raw.get("objectives", []),
        root=root,
        actions=[Action(**a) for a in raw.get("actions", [])],
        inspect=[Inspect(**i) for i in raw.get("inspect", [])],
        run=raw.get("run", []),
        cleanup=raw.get("cleanup", []),
        diagram=raw.get("diagram", {}) or {},
        failure_modes=raw.get("failure_modes", []) or [],
        expect=raw.get("expect", {}) or {},
        settings=raw.get("settings", {}) or {},
    )


class Registry:
    """Every scenario.yaml under the scenarios folder.

    Reloaded as soon as a descriptor changes on disk, so editing a scenario
    takes effect on the next request without restarting the dashboard, and
    without stopping the apps it is running: those belong to the process
    manager, keyed by scenario and action id, and survive a reload. A file
    that fails to parse mid-edit keeps its last good version and is reported
    in the dashboard log."""

    def __init__(self, base: Path = SCENARIOS_DIR):
        self.base = base
        self._scenarios: dict = {}
        self._by_path: dict = {}
        self._fingerprint: tuple = ()
        self.reload()

    def _scan(self) -> tuple:
        out = []
        for descriptor in sorted(self.base.glob("*/scenario.yaml")):
            try:
                out.append((str(descriptor), descriptor.stat().st_mtime_ns))
            except FileNotFoundError:
                pass
        return tuple(out)

    def reload(self) -> None:
        fingerprint = self._scan()
        by_path = {}
        for path, _ in fingerprint:
            try:
                by_path[path] = _load_one(Path(path))
            except Exception as exc:
                previous = self._by_path.get(path)
                kept = "keeping the last good version" if previous else "skipping it"
                log.warning("could not load %s (%s); %s", path, exc, kept)
                if previous:
                    by_path[path] = previous
        self._by_path = by_path
        self._scenarios = {s.id: s for s in by_path.values()}
        self._fingerprint = fingerprint

    def refresh(self) -> None:
        """Reload if any descriptor was added, removed or changed."""
        if self._scan() != self._fingerprint:
            self.reload()
            log.info("scenarios reloaded: %s", ", ".join(sorted(self._scenarios)))

    @property
    def fingerprint(self) -> tuple:
        return self._fingerprint

    def all(self) -> list:
        self.refresh()
        return sorted(self._scenarios.values(), key=lambda s: (s.order, s.title))

    def get(self, scenario_id: str) -> Optional[Scenario]:
        self.refresh()
        return self._scenarios.get(scenario_id)


# The value chosen for each setting, by (scenario id, setting name). Held
# here rather than in the browser so every start uses it, whoever presses
# Play: the page, a Break it card's reset or smoke.py. Forgotten when the
# dashboard restarts, or when the scenario is reset.
_setting_values: dict = {}


def setting_value(scenario: Scenario, name: str) -> str:
    spec = scenario.settings.get(name) or {}
    return _setting_values.get((scenario.id, name), spec.get("default"))


def set_setting(scenario: Scenario, name: str, value: str) -> None:
    spec = scenario.settings.get(name)
    if spec is None:
        raise ValueError(f"{scenario.id} has no setting {name!r}")
    if value not in [c["value"] for c in spec.get("choices", [])]:
        raise ValueError(f"{value!r} is not a choice for {name}")
    _setting_values[(scenario.id, name)] = value


def forget_settings(scenario_id: str) -> None:
    for key in [k for k in _setting_values if k[0] == scenario_id]:
        del _setting_values[key]


def uses_setting(action: Action, name: str) -> bool:
    return f"{{setting:{name}}}" in (action.cmd or [])


def _expand(scenario: Scenario, cmd: list) -> list:
    """Replace each "{setting:<name>}" with the chosen choice's arguments."""
    out = []
    for part in cmd:
        if isinstance(part, str) and part.startswith("{setting:") and part.endswith("}"):
            name = part[len("{setting:"):-1]
            value = setting_value(scenario, name)
            choice = next((c for c in scenario.settings.get(name, {}).get("choices", [])
                           if c["value"] == value), None)
            if choice is None:
                raise ValueError(f"setting {name!r} has no choice {value!r}")
            out.extend(str(a) for a in choice.get("args", []))
        else:
            out.append(part)
    return out


def build_command(scenario: Scenario, action: Action):
    """Resolve an action into (argv, cwd, env) ready for the ProcessManager.

    Terraform runs with state held under cockpit/.state/<scenario>/ rather than
    beside the .tf files, so a reset is a directory delete and the repo stays
    clean for the next attendee.
    """
    if action.kind == "terraform":
        workdir = scenario.tf_workdir(action)
        state = scenario_state_dir(scenario.id)
        state.mkdir(parents=True, exist_ok=True)
        statefile = state / f"{action.tf_dir}.tfstate"

        if action.command == "apply":
            argv = ["terraform", f"-chdir={workdir}", "apply",
                    "-auto-approve", "-no-color", f"-state={statefile}"]
        elif action.command == "destroy":
            argv = ["terraform", f"-chdir={workdir}", "destroy",
                    "-auto-approve", "-no-color", f"-state={statefile}"]
        elif action.command == "plan":
            argv = ["terraform", f"-chdir={workdir}", "plan",
                    "-no-color", f"-state={statefile}"]
        else:
            raise ValueError(f"unsupported terraform command: {action.command}")
        return argv, str(workdir), terraform_env()

    if action.kind == "process":
        cwd = scenario.root / action.cwd if action.cwd else scenario.root
        # Process actions get the full environment, terraform variables included.
        # Sample apps need the broker connection details, and helper scripts that
        # shell out to terraform need the TF_VAR_* values and the state location.
        # Without them a script would silently target a different broker, or a
        # different state file, than the cockpit does.
        return _expand(scenario, list(action.cmd)), str(cwd), terraform_env()

    raise ValueError(f"unsupported action kind: {action.kind}")


registry = Registry()
