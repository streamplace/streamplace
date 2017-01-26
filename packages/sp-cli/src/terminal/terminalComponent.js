
import {terminal as term} from "terminal-kit";

export default function terminalRender(store) {
  // term.fullscreen();
  term.alternateScreenBuffer();

  store.subscribe(() => {
    const {title, status, entries, categories} = store.getState().terminal;
    term.clear();

    term.moveTo(1, term.height);
    entries.forEach((entry) => {
      const cat = categories[entry.category];
      term.colorRgb(...cat.color)(`${cat.name} `);
      term.colorRgb(255, 255, 255)(`${entry.text}\n`);
    });

    term.saveCursor();
    // Set up top variables
    const titleText = ` ${title.text} `;
    const statusText = ` ${status.text} `;
    const neededSpacing = term.width - titleText.length - statusText.length;
    let spacing = "";
    while (spacing.length < neededSpacing) {
      spacing += " ";
    }

    // Print top bar
    term.moveTo(1, 1);
    term.underline.colorRgb(...title.color)(titleText);
    term.underline.colorRgb(255, 255, 255)(spacing);
    term.underline.colorRgb(...status.color)(statusText);

    term.restoreCursor();
  });
}
