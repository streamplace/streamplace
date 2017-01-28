
import {terminal as term} from "terminal-kit";

const hasPrinted = new Set();

export default function terminalRender(store) {
  // term.fullscreen();
  // term.alternateScreenBuffer();

  store.subscribe(() => {
    const {title, status, entries, categories} = store.getState().terminal;

    term.eraseDisplayBelow();

    // This is a memory leak, woo
    entries.forEach((entry) => {
      if (hasPrinted.has(entry)) {
        return;
      }
      hasPrinted.add(entry);
      const cat = categories[entry.category];
      term.colorRgb(...cat.color)(`${cat.name} `);
      term.colorRgb(255, 255, 255)(`${entry.text}\n`);
    });

    // Set up bottom bar
    const titleText = ` ${title.text} `;
    const statusText = ` ${status.text} `;
    const neededSpacing = term.width - titleText.length - statusText.length;
    let spacing = "";
    while (spacing.length < neededSpacing) {
      spacing += " ";
    }
    term("\n");
    term.up(1);

    term.down(1);
    term.underline.colorRgb(...title.color)(titleText);
    term.underline.colorRgb(255, 255, 255)(spacing);
    term.underline.colorRgb(...status.color)(statusText);
    term.up(1);
    term.column(1);
    term.styleReset();
  });
}
