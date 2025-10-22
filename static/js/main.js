document.addEventListener("DOMContentLoaded", function () {
  initDeviceButtons();
  setupPreviewIframe();
  initCopyContentButton();
  initBreadcrumbDownload();
  initExportButton();
  initSendEmailButton();
  highlightActiveNode();
  highlightCurrentLocation();
  initResizeTracking();
  initWidthSlider();
  initNavigationTree();

  // Watch for content changes to re-initialize breadcrumb download
  observeContentChanges();
});

// Function to highlight current location in directory listing
function highlightCurrentLocation() {
  // Add a class to the current directory in the main directory listing
  const currentPathDisplay = document.querySelector(".directory-header h2");
  if (currentPathDisplay) {
    // Add a visual indicator of current location
    currentPathDisplay.classList.add("current-location");
  }
}

// Initialize resize observer to track resize container changes
// Function to initialize resizable preview window
function initResizeTracking() {
  const resizeContainer = document.querySelector(".resize-container");
  const sizeDisplay = document.getElementById("size-display");

  if (!resizeContainer || !sizeDisplay) return;

  // Initially update the size display
  updateSizeDisplay();

  // Create a ResizeObserver to watch for changes to the container size
  const resizeObserver = new ResizeObserver((entries) => {
    // Use requestAnimationFrame for smoother updates
    requestAnimationFrame(() => {
      for (const entry of entries) {
        if (entry.target === resizeContainer) {
          const width = Math.round(entry.contentRect.width);
          sizeDisplay.textContent = `Viewport: ${width}px width`;

          // Update slider value if it doesn't match
          const slider = document.getElementById("widthSlider");
          if (slider && parseInt(slider.value) !== width) {
            slider.value = width;
          }

          // Reset any active device button when manually resized
          const activeButton = document.querySelector(".device-btn.active");
          if (activeButton) {
            const btnWidth = activeButton.getAttribute("data-width");
            // Check if button width (e.g., "375px") matches current container width
            if (
              btnWidth !== width + "px" &&
              (btnWidth !== "100%" ||
                width !== resizeContainer.parentElement.offsetWidth)
            ) {
              activeButton.classList.remove("active");
            }
          }
        }
      }
    });
  });

  // Start observing
  if (resizeContainer) {
    resizeObserver.observe(resizeContainer);
  }
}

// Function to initialize the width slider
function initWidthSlider() {
  const slider = document.getElementById("widthSlider");
  const resizeContainer = document.getElementById("resizeContainer");
  const sizeDisplay = document.getElementById("size-display");

  if (!slider || !resizeContainer || !sizeDisplay) return;

  // Set initial slider value based on container width
  slider.value = resizeContainer.offsetWidth;

  slider.addEventListener("input", function () {
    const newWidth = this.value;
    resizeContainer.style.width = `${newWidth}px`;
    sizeDisplay.textContent = `Viewport: ${newWidth}px width`;

    // Deselect any active device button
    const activeButton = document.querySelector(".device-btn.active");
    if (activeButton) {
      activeButton.classList.remove("active");
    }
  });
}

// Function to update size display
// Function to highlight the active node in the navigation tree
function highlightActiveNode() {
  // Find active node and scroll to it
  const activeNode = document.querySelector(".tree-node.active");
  if (activeNode) {
    // Add a small delay to ensure DOM is ready
    setTimeout(() => {
      // Get parent nodes to ensure they're visible
      let parent = activeNode.parentElement;
      while (parent && !parent.classList.contains("navigation-tree")) {
        if (parent.classList.contains("tree-node")) {
          parent.classList.add("expanded");
        }
        parent = parent.parentElement;
      }

      // Scroll the active node into view
      //activeNode.scrollIntoView({ behavior: "smooth", block: "center" });
    }, 200);
  }
}

// Function to update size display
function updateSizeDisplay() {
  const resizeContainer = document.getElementById("resizeContainer"); // Use ID for consistency
  const sizeDisplay = document.getElementById("size-display");

  if (!resizeContainer || !sizeDisplay) return;

  const width = Math.round(resizeContainer.offsetWidth);
  const currentWidthText = `${width}px width`;

  // Update slider value if it exists
  const slider = document.getElementById("widthSlider");
  if (slider && parseInt(slider.value) !== width) {
    slider.value = width;
  }

  // Check if it's a "Full" size button that is active
  const fullButton = document.getElementById("full-btn");
  if (fullButton && fullButton.classList.contains("active")) {
    sizeDisplay.textContent = `Viewport: Full size`;
  } else {
    sizeDisplay.textContent = `Viewport: ${currentWidthText}`;
  }
}

// Function to initialize device preview buttons
function initDeviceButtons() {
  const deviceButtons = document.querySelectorAll(".device-btn");
  if (deviceButtons.length === 0) return;

  deviceButtons.forEach((btn) => {
    btn.addEventListener("click", function () {
      // Update active state
      deviceButtons.forEach((b) => b.classList.remove("active"));
      this.classList.add("active");

      // Get the resize container
      const resizeContainer = document.getElementById("resizeContainer"); // Use ID
      const iframe = document.getElementById("preview-frame");
      if (!resizeContainer || !iframe) return;

      const width = this.getAttribute("data-width");
      const height = this.getAttribute("data-height");

      // Update the dimensions display
      const sizeDisplay = document.getElementById("size-display");
      if (sizeDisplay) {
        if (width === "100%") {
          sizeDisplay.textContent = `Viewport: Full size`;
        } else {
          sizeDisplay.textContent = `Viewport: ${width}`;
        }
      }

      // Set container width removing 'px' if it exists
      if (width === "100%") {
        resizeContainer.style.width = "100%";
        // Ensure the slider reflects this state if it exists
        const slider = document.getElementById("widthSlider");
        if (slider) {
          // Set slider to a representative value for "100%" or disable it
          // For now, let's set it to the container's current offsetWidth
          slider.value = resizeContainer.offsetWidth;
        }
      } else {
        const numWidth = parseInt(width);
        resizeContainer.style.width = `${numWidth}px`;
        // Update slider value
        const slider = document.getElementById("widthSlider");
        if (slider) {
          slider.value = numWidth;
        }
      }

      // Set container height
      resizeContainer.style.height = height;

      // Update the size display
      updateSizeDisplay();

      // Immediately dispatch resize event
      window.dispatchEvent(new Event("resize"));

      // Center the container if it's not full width
      const frameContainer = document.querySelector(".preview-frame-outer"); // Changed to outer container
      if (frameContainer) {
        if (width === "100%") {
          resizeContainer.style.margin = "0"; // Align to left for full width
          frameContainer.style.justifyContent = "flex-start";
        } else {
          resizeContainer.style.margin = "0 auto"; // Center for fixed widths
          frameContainer.style.justifyContent = "center";
        }
      }
    });
  });
}

// Function to set up the preview iframe
function setupPreviewIframe() {
  const iframe = document.getElementById("preview-frame");
  if (!iframe) return;

  // Load content via src to ensure template processing
  const baseUrl = window.location.pathname.replace("/preview/", "/raw/");
  iframe.src = baseUrl;

  // Force layout recalculation after load
  iframe.onload = () => {
    iframe.getBoundingClientRect();
  };

  // Set initial state of the resize container
  const resizeContainer = document.getElementById("resizeContainer");
  if (resizeContainer) {
    updateSizeDisplay();
    // Set initial slider value
    const slider = document.getElementById("widthSlider");
    if (slider) {
      slider.value = resizeContainer.offsetWidth;
    }
  }
}

// Function to initialize the copy content button
function initCopyContentButton() {
  const previewContainer = document.querySelector(".preview-container");
  if (!previewContainer) return;

  // Create copy button if it doesn't exist
  if (!document.getElementById("copy-content-btn")) {
    const copyBtn = document.createElement("button");
    copyBtn.id = "copy-content-btn";
    copyBtn.className = "action-btn";
    copyBtn.innerHTML =
      '<span class="action-icon">📋</span><span>Copy HTML</span>';

    // Insert the button in preview controls
    const previewControls = document.querySelector(".preview-controls");
    if (previewControls) {
      previewControls.appendChild(copyBtn);

      // Add click handler
      copyBtn.addEventListener("click", async function () {
        try {
          // Get current template path from URL
          const currentPath = window.location.pathname;
          const templatePath = currentPath.replace("/preview/", "");

          // Fetch both processed and unprocessed content
          const rawResponse = await fetch("/raw/" + templatePath);
          const originalResponse = await fetch("/original/" + templatePath);

          if (!rawResponse.ok || !originalResponse.ok) {
            throw new Error("Failed to fetch template content");
          }

          // Get both contents
          const rawContent = await rawResponse.text();
          const originalContent = await originalResponse.text();

          // Find all src and href attributes in raw content and map them back to BaseURL
          let finalContent = originalContent;
          const matches = rawContent.matchAll(
            /(?:src|href)="(\/templates\/[^"]+)"/g,
          );
          for (const match of matches) {
            const processedPath = match[1];
            const relativePath = processedPath.replace("/templates/", "");
            finalContent = finalContent.replace(
              processedPath,
              `{{.BaseURL}}/${relativePath}`,
            );
          }

          // Copy to clipboard
          await navigator.clipboard.writeText(finalContent);

          // Show success state
          const originalText = copyBtn.innerHTML;
          copyBtn.innerHTML =
            '<span class="action-icon">✅</span><span>Copied!</span>';
          copyBtn.classList.add("success");

          // Reset after 2 seconds
          setTimeout(() => {
            copyBtn.innerHTML = originalText;
            copyBtn.classList.remove("success");
          }, 2000);
        } catch (err) {
          console.error("Could not copy text: ", err);
          alert("Failed to copy HTML. Please try again.");
        }
      });
    }
  }
}

// Function to initialize breadcrumb download button
function initBreadcrumbDownload() {
  // Add listener for breadcrumb download button
  attachBreadcrumbDownloadListener();
}

// Function to observe content changes and re-attach breadcrumb download listener
function observeContentChanges() {
  // Use MutationObserver to detect when new content is loaded
  const targetNode = document.querySelector(".main-content");
  if (!targetNode) return;

  const config = { childList: true, subtree: true };

  const callback = function (mutationsList, observer) {
    for (let mutation of mutationsList) {
      if (mutation.type === "childList" && mutation.addedNodes.length > 0) {
        // Check if any breadcrumb download buttons were added
        const hasDownloadButtons = Array.from(mutation.addedNodes).some(
          (node) => {
            return (
              node.nodeType === Node.ELEMENT_NODE &&
              ((node.querySelector &&
                node.querySelector(".download-current-btn")) ||
                (node.classList &&
                  node.classList.contains("download-current-btn")))
            );
          },
        );

        if (hasDownloadButtons) {
          // Small delay to ensure DOM is fully updated
          setTimeout(() => {
            attachBreadcrumbDownloadListener();
            // Re-initialize Lucide icons for new content
            if (typeof lucide !== "undefined") {
              lucide.createIcons();
            }
          }, 100);
        }
      }
    }
  };

  const observer = new MutationObserver(callback);
  observer.observe(targetNode, config);
}

// Function to attach listener to breadcrumb download button
function attachBreadcrumbDownloadListener() {
  const breadcrumbDownloadBtn = document.querySelector(".download-current-btn");

  if (
    breadcrumbDownloadBtn &&
    !breadcrumbDownloadBtn.hasAttribute("data-listener-attached")
  ) {
    const folderPath = breadcrumbDownloadBtn.getAttribute("data-path");

    if (folderPath) {
      breadcrumbDownloadBtn.addEventListener("click", function (e) {
        e.preventDefault();
        e.stopPropagation();

        // Add loading state
        const originalContent = breadcrumbDownloadBtn.innerHTML;
        breadcrumbDownloadBtn.innerHTML =
          '<span class="download-icon">⏳</span><span>Downloading...</span>';
        breadcrumbDownloadBtn.disabled = true;

        requestFolderDownload(folderPath).finally(() => {
          // Restore original button state
          breadcrumbDownloadBtn.innerHTML = originalContent;
          breadcrumbDownloadBtn.disabled = false;
        });
      });

      // Mark as having listener attached
      breadcrumbDownloadBtn.setAttribute("data-listener-attached", "true");
    }
  }
}

// Function to initialize email button and modal

// Function to request folder download
function requestFolderDownload(path) {
  // The path might already be URL encoded from the href attribute
  // Check if it's encoded and handle accordingly
  const isEncoded = path !== decodeURIComponent(path);

  let finalPath;
  if (isEncoded) {
    // If it's already encoded, use it as-is
    finalPath = path;
  } else {
    // If it's not encoded, encode it
    finalPath = encodeURIComponent(path);
  }

  const downloadUrl = `/api/download?path=${finalPath}`;

  // Create a temporary link element to trigger the download
  const downloadLink = document.createElement("a");
  downloadLink.href = downloadUrl;
  downloadLink.download = "";
  downloadLink.target = "_blank";

  // Append to document, trigger click, then remove
  document.body.appendChild(downloadLink);
  downloadLink.click();
  document.body.removeChild(downloadLink);
}

// Initialize navigation tree functionality
function initNavigationTree() {
  setupFolderToggling();
  applyStoredCollapseStates();
  scrollToDeepestExpanded();
}

function applyStoredCollapseStates() {
  const navTree = document.getElementById("navTree");
  if (!navTree) return;

  // Apply any stored collapse states
  const treeNodes = navTree.querySelectorAll(".tree-node[data-path]");
  treeNodes.forEach((node) => {
    const path = node.getAttribute("data-path");
    const wasCollapsed = localStorage.getItem("collapsed_" + path) === "true";

    if (wasCollapsed && node.classList.contains("expanded")) {
      const toggle = node.querySelector(".tree-node-header .tree-node-toggle");
      const children = node.querySelector(".tree-node-children");

      node.classList.remove("expanded");
      if (toggle) toggle.textContent = "►";
      if (children) {
        children.style.display = "none";
      }
    }
  });
}

// Setup folder toggling functionality
function setupFolderToggling() {
  const navTree = document.getElementById("navTree");
  if (!navTree) return;

  // Use event delegation for better performance and to handle dynamically added elements
  navTree.addEventListener("click", function (e) {
    // Check if clicked on toggle arrow
    const toggle = e.target.closest(".tree-node-toggle");
    if (toggle) {
      const treeNode = toggle.closest(".tree-node");
      // Check if this is a directory by looking for folder icon
      const isDirectory =
        treeNode &&
        treeNode.querySelector(".tree-node-icon") &&
        treeNode.querySelector(".tree-node-icon").textContent.trim() === "📁";
      if (isDirectory) {
        e.preventDefault();
        e.stopPropagation();
        toggleFolder(treeNode);
        return;
      }
    }

    // Check if clicked on folder header (but not on a link)
    const header = e.target.closest(".tree-node-header");
    if (header && !e.target.closest("a")) {
      const treeNode = header.closest(".tree-node");
      // Check if this is a directory by looking for folder icon
      const isDirectory =
        treeNode &&
        treeNode.querySelector(".tree-node-icon") &&
        treeNode.querySelector(".tree-node-icon").textContent.trim() === "📁";
      if (isDirectory) {
        e.preventDefault();
        toggleFolder(treeNode);
        return;
      }
    }
  });

  // Make folder headers look clickable
  const folderHeaders = navTree.querySelectorAll(
    ".tree-node[data-path] > .tree-node-header",
  );
  folderHeaders.forEach((header) => {
    const treeNode = header.parentElement;
    // Check if this is a directory by looking for folder icon
    const isDirectory =
      treeNode.querySelector(".tree-node-icon") &&
      treeNode.querySelector(".tree-node-icon").textContent.trim() === "📁";
    if (isDirectory) {
      header.style.cursor = "pointer";
    }
  });
}

// Toggle folder expanded/collapsed state
function toggleFolder(treeNode) {
  const isExpanded = treeNode.classList.contains("expanded");
  const toggle = treeNode.querySelector(".tree-node-header .tree-node-toggle");
  const children = treeNode.querySelector(".tree-node-children");
  const folderPath = treeNode.getAttribute("data-path");

  if (isExpanded) {
    // Collapse the folder locally
    treeNode.classList.remove("expanded");
    if (toggle) toggle.textContent = "►";
    if (children) {
      children.style.display = "none";
    }

    // Store collapsed state to prevent re-expansion on navigation
    localStorage.setItem("collapsed_" + folderPath, "true");
  } else {
    // Check if this folder was previously collapsed
    const wasCollapsed =
      localStorage.getItem("collapsed_" + folderPath) === "true";

    if (wasCollapsed || !children) {
      // Navigate to folder to get server-rendered children
      localStorage.removeItem("collapsed_" + folderPath);
      window.location.href = "/" + folderPath;
    } else {
      // Expand locally if children already exist
      treeNode.classList.add("expanded");
      if (toggle) toggle.textContent = "▼";
      if (children) {
        children.style.display = "block";
      }
    }
  }
}

// Find and scroll to the deepest expanded folder
function scrollToDeepestExpanded() {
  const sidebar = document.querySelector(".sidebar");
  const navTree = document.getElementById("navTree");
  if (!sidebar || !navTree) {
    // If no navTree, just show it
    if (navTree) navTree.classList.add("loaded");
    return;
  }

  // Find the active node first
  const activeNode = document.querySelector(".tree-node.active");
  if (activeNode) {
    scrollToNode(activeNode, sidebar, true);
    showNavTree(navTree);
    return;
  }

  // Otherwise find deepest expanded node
  const expandedNodes = document.querySelectorAll(".tree-node.expanded");
  if (expandedNodes.length === 0) {
    showNavTree(navTree);
    return;
  }

  let deepestNode = null;
  let maxDepth = 0;

  expandedNodes.forEach((node) => {
    let depth = 0;
    let parent = node.parentElement;

    while (parent && parent !== navTree) {
      if (parent.classList.contains("tree-node-children")) {
        depth++;
      }
      parent = parent.parentElement;
    }

    if (depth > maxDepth) {
      maxDepth = depth;
      deepestNode = node;
    }
  });

  if (deepestNode) {
    scrollToNode(deepestNode, sidebar, true);
  }

  showNavTree(navTree);
}

// Show navigation tree after positioning
function showNavTree(navTree) {
  navTree.classList.add("loaded");
}

// Scroll a specific node into view
function scrollToNode(node, sidebar, instant = false) {
  const nodeRect = node.getBoundingClientRect();
  const sidebarRect = sidebar.getBoundingClientRect();

  // Calculate the position to scroll to (show node at the top of the sidebar)
  const scrollTop = sidebar.scrollTop + nodeRect.top - sidebarRect.top - 20; // 20px padding from top

  // Use instant scrolling for page load, smooth for user interactions
  if (instant) {
    sidebar.scrollTop = Math.max(0, scrollTop);
  } else {
    sidebar.scrollTo({
      top: Math.max(0, scrollTop),
      behavior: "smooth",
    });
  }
}

// Initialize export button functionality
function initExportButton() {
  const exportButton = document.getElementById("exportButton");
  if (exportButton) {
    exportButton.addEventListener("click", function () {
      requestExport();
    });
  }
}

// Function to request export
function requestExport() {
  // Show loading state
  const exportButton = document.getElementById("exportButton");
  const originalText = exportButton.innerHTML;
  exportButton.innerHTML = "⏳ Exporting...";
  exportButton.disabled = true;

  // Create the URL for the export endpoint
  const exportUrl = "/api/export";

  // Create a temporary link element to trigger the download
  const downloadLink = document.createElement("a");
  downloadLink.href = exportUrl;
  downloadLink.target = "_blank";

  // Append to document, trigger click, then remove
  document.body.appendChild(downloadLink);
  downloadLink.click();
  document.body.removeChild(downloadLink);

  // Reset button state after a short delay
  setTimeout(() => {
    exportButton.innerHTML = originalText;
    exportButton.disabled = false;
  }, 2000);
}

// Email functionality
function initSendEmailButton() {
  const sendEmailBtn = document.getElementById("sendEmailBtn");
  if (sendEmailBtn) {
    // Check if this is an email template
    checkIfEmailTemplate();

    sendEmailBtn.addEventListener("click", function () {
      sendTestEmail();
    });
  }
}

function checkIfEmailTemplate() {
  const templateData = document.getElementById("template-data");
  if (!templateData) return;

  const data = JSON.parse(templateData.textContent);
  const templatePath = data.path;

  fetch(`/api/check-email-template?path=${encodeURIComponent(templatePath)}`)
    .then((response) => response.json())
    .then((data) => {
      const sendEmailBtn = document.getElementById("sendEmailBtn");
      const mailpitBtn = document.querySelector(".mailpit-btn");

      if (data.isEmail && sendEmailBtn) {
        sendEmailBtn.style.display = "flex";

        // Show Mailpit button as well
        if (mailpitBtn) {
          mailpitBtn.style.display = "flex";
        }

        // Update button with email config if available
        if (data.emailConfig) {
          sendEmailBtn.title = `Send "${data.emailConfig.subject}" from ${data.emailConfig.from}`;
        }
      }
    })
    .catch((error) => {
      console.error("Error checking email template:", error);
    });
}

function sendTestEmail() {
  const templateData = document.getElementById("template-data");
  if (!templateData) {
    alert("Template data not found");
    return;
  }

  const data = JSON.parse(templateData.textContent);
  const templatePath = data.path;

  // Show loading state
  const sendEmailBtn = document.getElementById("sendEmailBtn");
  const originalContent = sendEmailBtn.innerHTML;
  sendEmailBtn.innerHTML =
    '<span class="email-icon">⏳</span><span>Sending...</span>';
  sendEmailBtn.disabled = true;

  // Send the email with default recipient
  fetch("/api/send-test-email", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      templatePath: templatePath,
      to: "test@example.com",
    }),
  })
    .then((response) => response.json())
    .then((data) => {
      if (data.success) {
        showToast("📧 Email sent successfully!");
      } else {
        alert(`❌ Error: ${data.message}`);
      }
    })
    .catch((error) => {
      console.error("Error sending email:", error);
      alert("❌ Error sending email. Please check the console for details.");
    })
    .finally(() => {
      // Reset button state
      sendEmailBtn.innerHTML = originalContent;
      sendEmailBtn.disabled = false;
    });
}

// Toast notification function
function showToast(message) {
  // Remove any existing toast
  const existingToast = document.querySelector(".toast");
  if (existingToast) {
    existingToast.remove();
  }

  // Create toast element
  const toast = document.createElement("div");
  toast.className = "toast";
  toast.textContent = message;

  // Add to document
  document.body.appendChild(toast);

  // Auto-hide after 3 seconds
  setTimeout(() => {
    toast.classList.add("toast-hide");
    setTimeout(() => {
      if (toast.parentNode) {
        toast.parentNode.removeChild(toast);
      }
    }, 300); // Match animation duration
  }, 3000);
}
