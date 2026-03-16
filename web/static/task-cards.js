document.addEventListener("DOMContentLoaded", () => {
  document.querySelectorAll("[data-complete-form]").forEach((form) => {
    form.addEventListener("submit", async (event) => {
      if (form.dataset.submitting === "1") {
        return;
      }

      event.preventDefault();
      form.dataset.submitting = "1";
      const submitButton = form.querySelector("button");
      if (submitButton) {
        submitButton.disabled = true;
      }

      const card = form.closest("[data-task-card]");
      try {
        const response = await fetch(form.action, {
          method: "POST",
          body: new FormData(form),
          headers: {
            "X-Requested-With": "fetch",
          },
        });

        if (!response.ok) {
          throw new Error("request failed");
        }

        const payload = await response.json();
        if (card) {
          completeCard(card, payload);
        }
      } catch (_error) {
        form.submit();
      }
    });
  });
});

function completeCard(card, payload) {
  const archiveSection = document.querySelector("[data-archive-section]");
  const archiveList = document.querySelector("[data-archive-list]");
  const archiveCount = document.querySelector("[data-archive-count]");
  const focusCounter = document.querySelector(".focus-counter");
  const focusList = card.parentElement;

  card.classList.add("is-completing");

  window.setTimeout(() => {
    if (archiveSection && archiveList) {
      archiveSection.classList.remove("is-empty");
      archiveList.prepend(buildArchiveCard(payload));
      if (archiveCount) {
        const count = Number.parseInt(archiveCount.textContent || "0", 10) || 0;
        archiveCount.textContent = String(count + 1);
      }
    }

    card.remove();

    if (focusCounter) {
      const count = Number.parseInt(focusCounter.textContent || "0", 10) || 0;
      focusCounter.textContent = String(Math.max(0, count - 1));
    }

    if (focusList && focusList.children.length === 0) {
      window.location.reload();
    }
  }, 120);
}

function buildArchiveCard(payload) {
  const article = document.createElement("article");
  article.className = "archive-card";
  article.innerHTML = `
    <div class="archive-card-main">
      <span class="task-kind task-kind-${escapeHtml(payload.kind_class)}">${escapeHtml(payload.kind_label)}</span>
      <div class="task-body">
        <h3>${escapeHtml(payload.title)}</h3>
        <p class="status">${escapeHtml(payload.finished_line)}</p>
        ${payload.note ? `<p class="note">${escapeHtml(payload.note)}</p>` : ""}
      </div>
    </div>
  `;
  return article;
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}
