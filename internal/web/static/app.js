async function fetchJSON(path, opts) {
  const res = await fetch(path, opts);
  return res.json();
}

function renderZones(profile) {
  const el = document.getElementById("zones");
  el.innerHTML = "";
  const zones = profile.zones || {};
  for (const [id, z] of Object.entries(zones)) {
    const card = document.createElement("div");
    card.className = "zone-card";
    card.innerHTML =
      "<strong>" + id + "</strong>" +
      "Mean: " + z.meanC.toFixed(1) + " °C<br>" +
      "Set: " + z.setpointC.toFixed(1) + " °C<br>" +
      "Sensors: " + z.sensorN;
    el.appendChild(card);
  }
}

function renderAlarms(alarms) {
  const el = document.getElementById("alarms");
  el.innerHTML = "";
  if (!alarms.length) {
    el.innerHTML = "<li>No active alarms</li>";
    return;
  }
  for (const a of alarms) {
    const li = document.createElement("li");
    li.className = a.severity === "critical" ? "alarm-critical" : "alarm-warn";
    li.textContent = "[" + a.code + "] " + a.message;
    el.appendChild(li);
  }
}

async function refresh() {
  try {
    const status = await fetchJSON("/v1/status");
    document.getElementById("status").textContent = JSON.stringify(status, null, 2);
    renderZones(status.profile || {});
    const alarms = await fetchJSON("/v1/alarms");
    renderAlarms(alarms);
  } catch (e) {
    document.getElementById("status").textContent = "Error: " + e.message;
  }
}

document.getElementById("btn-refresh").addEventListener("click", refresh);
document.getElementById("btn-estop").addEventListener("click", async () => {
  await fetchJSON("/v1/estop", { method: "POST" });
  refresh();
});
document.getElementById("btn-clear").addEventListener("click", async () => {
  await fetchJSON("/v1/estop/clear", { method: "POST" });
  refresh();
});

refresh();
setInterval(refresh, 5000);
