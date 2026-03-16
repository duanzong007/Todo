(() => {
  function activateComposerTab(root, nextMode) {
    root.querySelectorAll("[data-composer-tab]").forEach((button) => {
      button.classList.toggle("is-active", button.getAttribute("data-composer-tab") === nextMode);
    });

    root.querySelectorAll("[data-composer-section]").forEach((section) => {
      const active = section.getAttribute("data-composer-section") === nextMode;
      section.classList.toggle("is-active", active);
      section.hidden = !active;
    });
  }

  function bindComposerPanel(root) {
    if (root.dataset.composerBound === "1") {
      return;
    }
    root.dataset.composerBound = "1";

    root.querySelectorAll("[data-composer-tab]").forEach((button) => {
      button.addEventListener("click", () => {
        activateComposerTab(root, button.getAttribute("data-composer-tab") || "todo");
      });
    });

    root.querySelectorAll("[data-composer-date-shortcut]").forEach((button) => {
      button.addEventListener("click", () => {
        const form = button.closest("form");
        const picker = form?.querySelector("[data-composer-picker]");
        if (!picker || !window.setWheelPickerValue) {
          return;
        }

        const shortcutDate = button.getAttribute("data-shortcut-value") || "";
        const mode = picker.getAttribute("data-picker-mode") || "date";
        if (mode === "datetime") {
          const currentValue = picker.querySelector("[data-picker-value]")?.value || "";
          const timePart = currentValue.includes("T") ? currentValue.split("T")[1] : "08:00";
          window.setWheelPickerValue(picker, `${shortcutDate}T${timePart}`);
          return;
        }

        window.setWheelPickerValue(picker, shortcutDate);
      });
    });

    const icsButton = root.querySelector("[data-ics-upload-button]");
    const icsInput = root.querySelector("[data-ics-upload-input]");
    const icsForm = icsInput?.closest("form");
    if (icsButton && icsInput && icsForm) {
      icsButton.addEventListener("click", () => {
        icsInput.click();
      });
      icsInput.addEventListener("change", () => {
        if (!icsInput.files || icsInput.files.length === 0) {
          return;
        }
        if (window.initializeTaskCards) {
          window.initializeTaskCards(root);
        }
        if (typeof icsForm.requestSubmit === "function") {
          icsForm.requestSubmit();
          return;
        }
        icsForm.submit();
      });
    }
  }

  function initializeComposerPanels(scope = document) {
    scope.querySelectorAll("[data-composer-panel]").forEach((root) => {
      bindComposerPanel(root);
      const activeButton = root.querySelector("[data-composer-tab].is-active") || root.querySelector("[data-composer-tab]");
      activateComposerTab(root, activeButton?.getAttribute("data-composer-tab") || "todo");
    });
  }

  window.initializeComposerPanels = initializeComposerPanels;
  document.addEventListener("DOMContentLoaded", () => {
    initializeComposerPanels(document);
  });
})();
