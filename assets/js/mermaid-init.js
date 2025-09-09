// Mermaid initialization for GitHub Pages
document.addEventListener('DOMContentLoaded', function() {
  // Check if mermaid diagrams exist on the page
  const mermaidElements = document.querySelectorAll('.language-mermaid, pre code.language-mermaid, .mermaid');
  
  if (mermaidElements.length > 0) {
    // Dynamically load Mermaid if diagrams are present
    const script = document.createElement('script');
    script.type = 'module';
    script.textContent = `
      import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs';
      
      mermaid.initialize({
        startOnLoad: false,
        theme: 'default',
        themeVariables: {
          primaryColor: '#159957',
          primaryTextColor: '#fff',
          primaryBorderColor: '#155c5c',
          lineColor: '#155c5c',
          secondaryColor: '#155c5c',
          tertiaryColor: '#f0f0f0'
        },
        flowchart: {
          useMaxWidth: true,
          htmlLabels: true,
          curve: 'basis'
        },
        securityLevel: 'loose'
      });

      // Convert code blocks to mermaid diagrams
      document.querySelectorAll('pre code.language-mermaid').forEach((element, index) => {
        const pre = element.parentNode;
        const mermaidDiv = document.createElement('div');
        mermaidDiv.className = 'mermaid';
        mermaidDiv.textContent = element.textContent;
        mermaidDiv.id = 'mermaid-' + index;
        
        pre.parentNode.insertBefore(mermaidDiv, pre);
        pre.style.display = 'none';
      });

      // Render all mermaid diagrams
      mermaid.run();
    `;
    
    document.head.appendChild(script);
  }
});

// Fallback for older browsers or if ES modules are not supported
if (!window.mermaid) {
  const fallbackScript = document.createElement('script');
  fallbackScript.src = 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js';
  fallbackScript.onload = function() {
    mermaid.initialize({
      startOnLoad: true,
      theme: 'default',
      themeVariables: {
        primaryColor: '#159957',
        primaryTextColor: '#fff',
        primaryBorderColor: '#155c5c',
        lineColor: '#155c5c',
        secondaryColor: '#155c5c',
        tertiaryColor: '#f0f0f0'
      }
    });
  };
  document.head.appendChild(fallbackScript);
}