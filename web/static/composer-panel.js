(() => {
  function activateScheduleMode(form, nextMode) {
    const mode = nextMode === "batch" ? "batch" : "single";
    const hiddenInput = form.querySelector("[data-schedule-mode-input]");
    if (hiddenInput) {
      hiddenInput.value = mode;
    }

    form.querySelectorAll("[data-schedule-mode-button]").forEach((button) => {
      button.classList.toggle("is-active", button.getAttribute("data-schedule-mode-value") === mode);
    });

    form.querySelectorAll("[data-schedule-mode-panel]").forEach((panel) => {
      const active = panel.getAttribute("data-schedule-mode-panel") === mode;
      panel.hidden = !active;
    });
  }

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

    root.querySelectorAll("form[data-composer-section='schedule']").forEach((form) => {
      form.querySelectorAll("[data-schedule-mode-button]").forEach((button) => {
        button.addEventListener("click", () => {
          activateScheduleMode(form, button.getAttribute("data-schedule-mode-value") || "single");
        });
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

      root.querySelectorAll("form[data-composer-section='schedule']").forEach((form) => {
        const activeMode =
          form.querySelector("[data-schedule-mode-button].is-active")?.getAttribute("data-schedule-mode-value") ||
          form.querySelector("[data-schedule-mode-input]")?.value ||
          "single";
        activateScheduleMode(form, activeMode);
      });
    });
  }

  window.initializeComposerPanels = initializeComposerPanels;
  document.addEventListener("DOMContentLoaded", () => {
    initializeComposerPanels(document);
  });
})();
