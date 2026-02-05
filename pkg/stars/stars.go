package stars

// censors a message (with stars) based on regex patterns
import (
	"encoding/json"
	"os"
	"regexp"
	"strings"

	appbsky "github.com/bluesky-social/indigo/api/bsky"
	"stream.place/streamplace/pkg/streamplace"
)

// PatternDef defines a pattern with its associated categories
type PatternDef struct {
	Pattern    string
	Categories []string
}

// Default patterns for common profanity and slurs (case-insensitive)
var DefaultPatterns = []PatternDef{
	{`(?i)\b[cCϲсᴄⅽｃçćčĉċ¢©🅒🅲𝐜𝑐𝒄𝒸𝓬𝔠𝕔𝖈𝖼𝗰𝘤𝙘𝚌ⓒⒸᶜ\(\[\{<ⲥꮯ€🇨][uUυսｕùúûüũūŭůűųưᴜᵘᵤ🅤🆄𝐮𝑢𝒖𝓊𝓾𝔲𝕦𝖚𝗎𝘂𝘶𝙪𝚞ⓤⓊʋꞟꭎꭒ𑣘ט𑜆🇺𝖀vμ][nNոռｎñńņňŉṅṇṉṋɴⁿ🅝🅽𝐧𝑛𝒏𝓃𝓷𝔫𝕟𝖓𝗇𝗻𝘯𝙣𝚗ⓝⓃ🇳ηŋℕ𝕹][tTтｔţťŧṫṭṯṱᴛᵗ7\+🅣🆃𝐭𝑡𝒕𝓉𝓽𝔱𝕥𝖙𝗍𝘁𝘵𝙩𝚝ⓣⓉ†✝]+`, []string{"place.stream.richtext.defs#sexually_explicit", "place.stream.richtext.defs#profanity"}},
	{`(?i)\b(n|\|\\||🇳|ո|ռ|🅝|𝕹)+(i|1|!|\||l|🇮|ℹ️|ı|ɩ|ɪ|ӏ|Ꭵ|ꙇ|ꭵ|ǀ|Ι|І|Ӏ|׀|ו|ן|١|۱|ا|Ⲓ|ⵏ|ꓲ|𐊊|𐌉|𐌠|𖼨|ﺍ|ﺎ|￨|🅘|𝕴)+(g|9|🇬|ƍ|ɡ|ᶃ|🅖|𝕲)`, []string{"place.stream.richtext.defs#discriminatory"}},
	{`(?i)\b(f|ƒ|£|🇫|ẝ|ꞙ|ꬵ|🅕|𝖿|𝕱)+(a|4|@|∆|/-\\|/_\\|Д|🇦|🅰️|ɑ|а|🅐|𝖺|𝕬)+(g|9|🇬|ƍ|ɡ|ᶃ|🅖|𝕲)+`, []string{"place.stream.richtext.defs#discriminatory"}},
	{`(?i)\b[rRᎡꓣ𝐑𝐫𝑹𝒓ℛℜℝ𝓇𝓡𝓻𝔯𝕣𝕽𝖗𝖱𝗋𝗥𝗿𝘙𝘳𝙍𝙧𝚁𝚛ⓇⓡʀᴿʳŕŗřȑȓɍɹɻɼɽɾṙṛṝṟгГ®🅡🆁🄡][eEЕеᎬꓰ𝐄𝐞𝑬𝒆ℰℯ𝓔𝓮𝔈𝔢𝔼𝕖𝕰𝖊𝖤𝖾𝗘𝗲𝘌𝘦𝙀𝙚𝙴𝚎ⒺⓔᴇᴱᵉₑɛεΕёЁèéêëēĕėęěȅȇȩɇѐєҽ3€🅔🅴🄴℮ǝƎ∃][tTТтᎢꓔ𝐓𝐭𝑻𝒕𝒯𝓉𝓣𝓽𝔗𝔱𝕋𝕥𝕿𝖙𝖳𝗍𝗧𝘁𝘛𝘵𝙏𝙩𝚃𝚝Ⓣⓣᴛᵀᵗţťŧțȶṫṭṯṱ7\+†✝🅣🆃🄣][aAАаᎪꓮꭺ𐊠𝐀𝐚𝑨𝒂𝒜𝒶𝓐𝓪𝔄𝔞𝔸𝕒𝕬𝖆𝖠𝖺𝗔𝗮𝘈𝘢𝘼𝙖𝙰𝚊𝚨ⒶⓐᴀᴬᵃᵅₐɐɑαΑäàáâãåāăąǎǟǡǻȁȃȧӑӓᾀᾁᾂᾃᾄᾅᾆᾇᾰᾱᾲᾳᾴᾶᾷὰά@4🅐🅰🄰∀Λλ][rRᎡꓣ𝐑𝐫𝑹𝒓ℛℜℝ𝓇𝓡𝓻𝔯𝕣𝕽𝖗𝖱𝗋𝗥𝗿𝘙𝘳𝙍𝙧𝚁𝚛ⓇⓡʀᴿʳŕŗřȑȓɍɹɻɼɽɾṙṛṝṟгГ®🅡🆁🄡][dDᎠꓓ𝐃𝐝𝑫𝒅𝒟𝒹𝓓𝓭𝔇𝔡𝔻d𝕯𝖉𝖣𝖽𝗗𝗱𝘋𝘥𝘿𝙙𝙳𝚍ⒹⓓᴅᴰᵈԀԁɖɗďđðḋḍḏḑḓ🅓🅳🄳ⅅⅆ]+`, []string{"place.stream.richtext.defs#discriminatory"}},
	{`(?i)\b[bBВвᏴꓐ𐊂𐊡𐌁𝐁𝐛𝑩𝒃𝒷𝓑𝓫𝔅𝔟𝔹𝕓𝕭𝖇𝖡𝖻𝗕𝗯𝘉𝘣𝘽𝙗𝙱𝚋𝚩ⒷⓑᴃᴮᵇƀɓЬьβΒḃḅḇ68ßþ🅑🅱🄱ℬ][iIІіӀꓲ𝐈𝐢𝑰𝒊ℐℑ𝒾𝓘𝓲𝔦𝕀𝕚𝕴𝖎𝖨𝗂𝗜𝗶𝘐𝘪𝙄𝙞𝙸𝚒𝚤Ⓘⓘɪᴵⁱᵢìíîïĩīĭįıǐȉȋḭḯỉịӏ1l\|!¡🅘🅸🄸ⅰⅠ][tTТтᎢꓔ𝐓𝐭𝑻𝒕𝒯𝓉𝓣𝓽𝔗𝔱𝕋𝕥𝕿𝖙𝖳𝗍𝗧𝘁𝘛𝘵𝙏𝙩𝚃𝚝Ⓣⓣᴛᵀᵗţťŧțȶṫṭṯṱ7\+†✝🅣🆃🄣][cCСсᏟꓚꮯ𐊢𐌂𐐕𐐽𝐂𝐜𝑪𝒄𝒞𝒸𝓒𝓬ℭ𝔠ℂ𝕔𝕮𝖈𝖢𝖼𝗖𝗰𝘊𝘤𝘾𝙘𝙲𝚌ⒸⓒᴄↃↄϲϹçćĉċčƈȼ¢©€🅒🅲🄲⊂⊃ᑕᑢ\(<\[\{][hHНнᎻꓧ𝐇𝐡𝑯𝒉ℋℌℍ𝒽𝓗𝓱𝔥𝕙𝕳𝖍𝖧𝗁𝗛𝗵𝘏𝘩𝙃𝙝𝙷𝚑Ⓗⓗʜᴴʰĥħȟḣḥḧḩḫհңһ#🅗🅷🄷♄]+`, []string{"place.stream.richtext.defs#profanity"}},
	{`(?i)\b[dDԁɗḋḍḏḑḓｄᴅᵈ🅓🅳𝐝𝑑𝒅𝒹𝓭𝔡𝕕𝖉𝖽𝗱𝘥𝙙𝚍ⓓⒹđð][iIіıɪｉìíîïĩīĭįǐ1l\|!🅘🅸𝐢𝑖𝒊𝒾𝓲𝔦𝕚𝖎𝗂𝗶𝘪𝙞𝚒ⓘⒾᵢⁱ¡ǃ][cCϲсᴄⅽｃçćčĉċ¢©🅒🅲𝐜𝑐𝒄𝒸𝓬𝔠𝕔𝖈𝖼𝗰𝘤𝙘𝚌ⓒⒸᶜ\(\[\{<ⲥꮯ€🇨][kKκｋḱḳḵķᴋᵏ🅚🅺𝐤𝑘𝒌𝓀𝓴𝔨𝕜𝖐𝗄𝗸𝘬𝙠𝚔ⓚⓀꮶ]+`, []string{"place.stream.richtext.defs#sexually_explicit", "place.stream.richtext.defs#profanity"}},
	{`(?i)\b[pPрρｐṕṗᴘᵖ🅟🅿𝐩𝑝𝒑𝓅𝓹𝔭𝕡𝖕𝗉𝗽𝘱𝙥𝚙ⓟⓅ℘][uUυսｕùúûüũūŭůűųưᴜᵘᵤ🅤🆄𝐮𝑢𝒖𝓊𝓾𝔲𝕦𝖚𝗎𝘂𝘶𝙪𝚞ⓤⓊʋꞟꭎꭒ𑣘ט𑜆🇺𝖀vμ][sSѕꜱｓśŝşšṡṣṥṧṩˢ\$5🅢🆂𝐬𝑠𝒔𝓈𝓼𝔰𝕤𝖘𝗌𝘀𝘴𝙨𝚜ⓢⓈ§][sSѕꜱｓśŝşšṡṣṥṧṩˢ\$5🅢🆂𝐬𝑠𝒔𝓈𝓼𝔰𝕤𝖘𝗌𝘀𝘴𝙨𝚜ⓢⓈ§][yYуүγｙỳýŷÿỹȳɣʏყᶌỿℽꭚ𑣄¥🅨🆈𝐲𝑦𝒚𝓎𝔂𝔶𝕪𝖞𝗒𝘆𝘺𝙮𝚢ⓨⓎʸ🇾𝖄]+`, []string{"place.stream.richtext.defs#sexually_explicit", "place.stream.richtext.defs#profanity"}},
	{`(?i)\b(f|ƒ|£|🇫|ẝ|ꞙ|ꬵ|🅕|𝖿|𝕱)+(u|v|🇺|ʋ|υ|ս|ᴜ|ꞟ|ꭎ|ꭒ|𑣘|ט|𑜆|🅤|𝗎|𝖀)+(c|\(|€|🇨|©️|ϲ|с|ᴄ|ⲥ|ꮯ|🅒|𝖢|𝕮)+(k|\|<|🇰|🅚|𝕶)+`, []string{"place.stream.richtext.defs#profanity"}},
	// from https://github.com/bluesky-social/atproto/blob/7b9a98a763636c5f66a06da11fe6013f29dd9157/lexicons/app/bsky/richtext/facet.json
	{`/\b[cĆćĈĉČčĊċÇçḈḉȻȼꞒꞓꟄꞔƇƈɕ][hĤĥȞȟḦḧḢḣḨḩḤḥḪḫH̱ẖĦħⱧⱨꞪɦꞕΗНн][iÍíi̇́Ììi̇̀Ĭĭcccccbvnnuugtbekdkibdcrbceidjbticigulkbikbbl
  ÎîǏǐÏïḮḯĨĩi̇̃ĮįĮ́į̇́Į̃į̇̃ĪīĪ̀ī̀ỈỉȈȉI̋i̋ȊȋỊịꞼꞽḬḭƗɨᶖİiIıＩｉ1lĺľļḷḹl̃ḽḻłŀƚꝉⱡɫɬꞎꬷꬸꬹᶅɭȴＬｌ][nŃńǸǹŇňÑñṄṅŅņṆṇṊṋṈṉN̈n̈ƝɲŊŋꞐꞑꞤꞥᵰᶇɳȵꬻꬼИиПпＮｎ][kḰḱǨǩĶķḲḳḴḵƘƙⱩⱪᶄꝀꝁꝂꝃꝄꝅꞢꞣ][sŚśṤṥŜŝŠšṦṧṠṡŞşṢṣṨṩȘșS̩s̩ꞨꞩⱾȿꟅʂᶊᵴ]?\b/`, []string{"place.stream.richtext.defs#discriminatory"}},
	{`/\b[cĆćĈĉČčĊċÇçḈḉȻȼꞒꞓꟄꞔƇƈɕ][ÓóÒòŎŏÔôỐốỒồỖỗỔổǑǒÖöȪȫŐőÕõṌṍṎṏȬȭȮȯO͘o͘ȰȱØøǾǿǪǫǬǭŌōṒṓṐṑỎỏȌȍȎȏƠơỚớỜờỠỡỞởỢợỌọỘộO̩o̩Ò̩ò̩Ó̩ó̩ƟɵꝊꝋꝌꝍⱺＯｏ0]{2}[nŃńǸǹŇňÑñṄṅŅņṆṇṊṋṈṉN̈n̈ƝɲŊŋꞐꞑꞤꞥᵰᶇɳȵꬻꬼИиПпＮｎ][sŚśṤṥŜŝŠšṦṧṠṡŞşṢṣṨṩȘșS̩s̩ꞨꞩⱾȿꟅʂᶊᵴ]?\b/`, []string{"place.stream.richtext.defs#discriminatory"}},
	{`/\b[fḞḟƑƒꞘꞙᵮᶂ][aÁáÀàĂăẮắẰằẴẵẲẳÂâẤấẦầẪẫẨẩǍǎÅåǺǻÄäǞǟÃãȦȧǠǡĄąĄ́ą́Ą̃ą̃ĀāĀ̀ā̀ẢảȀȁA̋a̋ȂȃẠạẶặẬậḀḁȺⱥꞺꞻᶏẚＡａ@4][gǴǵĞğĜĝǦǧĠġG̃g̃ĢģḠḡǤǥꞠꞡƓɠᶃꬶＧｇ]{1,2}([ÓóÒòŎŏÔôỐốỒồỖỗỔổǑǒÖöȪȫŐőÕõṌṍṎṏȬȭȮȯO͘o͘ȰȱØøǾǿǪǫǬǭŌōṒṓṐṑỎỏȌȍȎȏƠơỚớỜờỠỡỞởỢợỌọỘộO̩o̩Ò̩ò̩Ó̩ó̩ƟɵꝊꝋꝌꝍⱺＯｏ0e3ЄєЕеÉéÈèĔĕÊêẾếỀềỄễỂểÊ̄ê̄Ê̌ê̌ĚěËëẼẽĖėĖ́ė́Ė̃ė̃ȨȩḜḝĘęĘ́ę́Ę̃ę̃ĒēḖḗḔḕẺẻȄȅE̋e̋ȆȇẸẹỆệḘḙḚḛɆɇE̩e̩È̩è̩É̩é̩ᶒⱸꬴꬳＥｅiÍíi̇́Ììi̇̀ĬĭÎîǏǐÏïḮḯĨĩi̇̃ĮįĮ́į̇́Į̃į̇̃ĪīĪ̀ī̀ỈỉȈȉI̋i̋ȊȋỊịꞼꞽḬḭƗɨᶖİiIıＩｉ1lĺľļḷḹl̃ḽḻłŀƚꝉⱡɫɬꞎꬷꬸꬹᶅɭȴＬｌ][tŤťṪṫŢţṬṭȚțṰṱṮṯŦŧȾⱦƬƭƮʈT̈ẗᵵƫȶ]{1,2}([rŔŕŘřṘṙŖŗȐȑȒȓṚṛṜṝṞṟR̃r̃ɌɍꞦꞧⱤɽᵲᶉꭉ][yÝýỲỳŶŷY̊ẙŸÿỸỹẎẏȲȳỶỷỴỵɎɏƳƴỾỿ]|[rŔŕŘřṘṙŖŗȐȑȒȓṚṛṜṝṞṟR̃r̃ɌɍꞦꞧⱤɽᵲᶉꭉ][iÍíi̇́Ììi̇̀ĬĭÎîǏǐÏïḮḯĨĩi̇̃ĮįĮ́į̇́Į̃į̇̃ĪīĪ̀ī̀ỈỉȈȉI̋i̋ȊȋỊịꞼꞽḬḭƗɨᶖİiIıＩｉ1lĺľļḷḹl̃ḽḻłŀƚꝉⱡɫɬꞎꬷꬸꬹᶅɭȴＬｌ][e3ЄєЕеÉéÈèĔĕÊêẾếỀềỄễỂểÊ̄ê̄Ê̌ê̌ĚěËëẼẽĖėĖ́ė́Ė̃ė̃ȨȩḜḝĘęĘ́ę́Ę̃ę̃ĒēḖḗḔḕẺẻȄȅE̋e̋ȆȇẸẹỆệḘḙḚḛɆɇE̩e̩È̩è̩É̩é̩ᶒⱸꬴꬳＥｅ])?)?[sŚśṤṥŜŝŠšṦṧṠṡŞşṢṣṨṩȘșS̩s̩ꞨꞩⱾȿꟅʂᶊᵴ]?\b/`, []string{"place.stream.richtext.defs#discriminatory"}},
	{`/\b[kḰḱǨǩĶķḲḳḴḵƘƙⱩⱪᶄꝀꝁꝂꝃꝄꝅꞢꞣ][iÍíi̇́Ììi̇̀ĬĭÎîǏǐÏïḮḯĨĩi̇̃ĮįĮ́į̇́Į̃į̇̃ĪīĪ̀ī̀ỈỉȈȉI̋i̋ȊȋỊịꞼꞽḬḭƗɨᶖİiIıＩｉ1lĺľļḷḹl̃ḽḻłŀƚꝉⱡɫɬꞎꬷꬸꬹᶅɭȴＬｌyÝýỲỳŶŷY̊ẙŸÿỸỹẎẏȲȳỶỷỴỵɎɏƳƴỾỿ][kḰḱǨǩĶķḲḳḴḵƘƙⱩⱪᶄꝀꝁꝂꝃꝄꝅꞢꞣ][e3ЄєЕеÉéÈèĔĕÊêẾếỀềỄễỂểÊ̄ê̄Ê̌ê̌ĚěËëẼẽĖėĖ́ė́Ė̃ė̃ȨȩḜḝĘęĘ́ę́Ę̃ę̃ĒēḖḗḔḕẺẻȄȅE̋e̋ȆȇẸẹỆệḘḙḚḛɆɇE̩e̩È̩è̩É̩é̩ᶒⱸꬴꬳＥｅ]([rŔŕŘřṘṙŖŗȐȑȒȓṚṛṜṝṞṟR̃r̃ɌɍꞦꞧⱤɽᵲᶉꭉ][yÝýỲỳŶŷY̊ẙŸÿỸỹẎẏȲȳỶỷỴỵɎɏƳƴỾỿ]|[rŔŕŘřṘṙŖŗȐȑȒȓṚṛṜṝṞṟR̃r̃ɌɍꞦꞧⱤɽᵲᶉꭉ][iÍíi̇́Ììi̇̀ĬĭÎîǏǐÏïḮḯĨĩi̇̃ĮįĮ́į̇́Į̃į̇̃ĪīĪ̀ī̀ỈỉȈȉI̋i̋ȊȋỊịꞼꞽḬḭƗɨᶖİiIıＩｉ1lĺľļḷḹl̃ḽḻłŀƚꝉⱡɫɬꞎꬷꬸꬹᶅɭȴＬｌ][e3ЄєЕеÉéÈèĔĕÊêẾếỀềỄễỂểÊ̄ê̄Ê̌ê̌ĚěËëẼẽĖėĖ́ė́Ė̃ė̃ȨȩḜḝĘęĘ́ę́Ę̃ę̃ĒēḖḗḔḕẺẻȄȅE̋e̋ȆȇẸẹỆệḘḙḚḛɆɇE̩e̩È̩è̩É̩é̩ᶒⱸꬴꬳＥｅ])?[sŚśṤṥŜŝŠšṦṧṠṡŞşṢṣṨṩȘșS̩s̩ꞨꞩⱾȿꟅʂᶊᵴ]*\b/`, []string{"place.stream.richtext.defs#discriminatory"}},
	{`/\b[nŃńǸǹŇňÑñṄṅŅņṆṇṊṋṈṉN̈n̈ƝɲŊŋꞐꞑꞤꞥᵰᶇɳȵꬻꬼИиПпＮｎ][iÍíi̇́Ììi̇̀ĬĭÎîǏǐÏïḮḯĨĩi̇̃ĮįĮ́į̇́Į̃į̇̃ĪīĪ̀ī̀ỈỉȈȉI̋i̋ȊȋỊịꞼꞽḬḭƗɨᶖİiIıＩｉ1lĺľļḷḹl̃ḽḻłŀƚꝉⱡɫɬꞎꬷꬸꬹᶅɭȴＬｌoÓóÒòŎŏÔôỐốỒồỖỗỔổǑǒÖöȪȫŐőÕõṌṍṎṏȬȭȮȯO͘o͘ȰȱØøǾǿǪǫǬǭŌōṒṓṐṑỎỏȌȍȎȏƠơỚớỜờỠỡỞởỢợỌọỘộO̩o̩Ò̩ò̩Ó̩ó̩ƟɵꝊꝋꝌꝍⱺＯｏІіa4ÁáÀàĂăẮắẰằẴẵẲẳÂâẤấẦầẪẫẨẩǍǎÅåǺǻÄäǞǟÃãȦȧǠǡĄąĄ́ą́Ą̃ą̃ĀāĀ̀ā̀ẢảȀȁA̋a̋ȂȃẠạẶặẬậḀḁȺⱥꞺꞻᶏẚＡａ][gǴǵĞğĜĝǦǧĠġG̃g̃ĢģḠḡǤǥꞠꞡƓɠᶃꬶＧｇqꝖꝗꝘꝙɋʠ]{2}(l[e3ЄєЕеÉéÈèĔĕÊêẾếỀềỄễỂểÊ̄ê̄Ê̌ê̌ĚěËëẼẽĖėĖ́ė́Ė̃ė̃ȨȩḜḝĘęĘ́ę́Ę̃ę̃ĒēḖḗḔḕẺẻȄȅE̋e̋ȆȇẸẹỆệḘḙḚḛɆɇE̩e̩È̩è̩É̩é̩ᶒⱸꬴꬳＥｅ]t|[e3ЄєЕеÉéÈèĔĕÊêẾếỀềỄễỂểÊ̄ê̄Ê̌ê̌ĚěËëẼẽĖėĖ́ė́Ė̃ė̃ȨȩḜḝĘęĘ́ę́Ę̃ę̃ĒēḖḗḔḕẺẻȄȅE̋e̋ȆȇẸẹỆệḘḙḚḛɆɇE̩e̩È̩è̩É̩é̩ᶒⱸꬴꬳＥｅaÁáÀàĂăẮắẰằẴẵẲẳÂâẤấẦầẪẫẨẩǍǎÅåǺǻÄäǞǟÃãȦȧǠǡĄąĄ́ą́Ą̃ą̃ĀāĀ̀ā̀ẢảȀȁA̋a̋ȂȃẠạẶặẬậḀḁȺⱥꞺꞻᶏẚＡａ][rŔŕŘřṘṙŖŗȐȑȒȓṚṛṜṝṞṟR̃r̃ɌɍꞦꞧⱤɽᵲᶉꭉ]?|n[ÓóÒòŎŏÔôỐốỒồỖỗỔổǑǒÖöȪȫŐőÕõṌṍṎṏȬȭȮȯO͘o͘ȰȱØøǾǿǪǫǬǭŌōṒṓṐṑỎỏȌȍȎȏƠơỚớỜờỠỡỞởỢợỌọỘộO̩o̩Ò̩ò̩Ó̩ó̩ƟɵꝊꝋꝌꝍⱺＯｏ0][gǴǵĞğĜĝǦǧĠġG̃g̃ĢģḠḡǤǥꞠꞡƓɠᶃꬶＧｇqꝖꝗꝘꝙɋʠ]|[a4ÁáÀàĂăẮắẰằẴẵẲẳÂâẤấẦầẪẫẨẩǍǎÅåǺǻÄäǞǟÃãȦȧǠǡĄąĄ́ą́Ą̃ą̃ĀāĀ̀ā̀ẢảȀȁA̋a̋ȂȃẠạẶặẬậḀḁȺⱥꞺꞻᶏẚＡａ]?)?[sŚśṤṥŜŝŠšṦṧṠṡŞşṢṣṨṩȘșS̩s̩ꞨꞩⱾȿꟅʂᶊᵴ]?\b/`, []string{"place.stream.richtext.defs#discriminatory"}},
	{`/[nŃńǸǹŇňÑñṄṅŅņṆṇṊṋṈṉN̈n̈ƝɲŊŋꞐꞑꞤꞥᵰᶇɳȵꬻꬼИиПпＮｎ][iÍíi̇́Ììi̇̀ĬĭÎîǏǐÏïḮḯĨĩi̇̃ĮįĮ́į̇́Į̃į̇̃ĪīĪ̀ī̀ỈỉȈȉI̋i̋ȊȋỊịꞼꞽḬḭƗɨᶖİiIıＩｉ1lĺľļḷḹl̃ḽḻłŀƚꝉⱡɫɬꞎꬷꬸꬹᶅɭȴＬｌoÓóÒòŎŏÔôỐốỒồỖỗỔổǑǒÖöȪȫŐőÕõṌṍṎṏȬȭȮȯO͘o͘ȰȱØøǾǿǪǫǬǭŌōṒṓṐṑỎỏȌȍȎȏƠơỚớỜờỠỡỞởỢợỌọỘộO̩o̩Ò̩ò̩Ó̩ó̩ƟɵꝊꝋꝌꝍⱺＯｏІіa4ÁáÀàĂăẮắẰằẴẵẲẳÂâẤấẦầẪẫẨẩǍǎÅåǺǻÄäǞǟÃãȦȧǠǡĄąĄ́ą́Ą̃ą̃ĀāĀ̀ā̀ẢảȀȁA̋a̋ȂȃẠạẶặẬậḀḁȺⱥꞺꞻᶏẚＡａ][gǴǵĞğĜĝǦǧĠġG̃g̃ĢģḠḡǤǥꞠꞡƓɠᶃꬶＧｇqꝖꝗꝘꝙɋʠ]{2}(l[e3ЄєЕеÉéÈèĔĕÊêẾếỀềỄễỂểÊ̄ê̄Ê̌ê̌ĚěËëẼẽĖėĖ́ė́Ė̃ė̃ȨȩḜḝĘęĘ́ę́Ę̃ę̃ĒēḖḗḔḕẺẻȄȅE̋e̋ȆȇẸẹỆệḘḙḚḛɆɇE̩e̩È̩è̩É̩é̩ᶒⱸꬴꬳＥｅ]t|[e3ЄєЕеÉéÈèĔĕÊêẾếỀềỄễỂểÊ̄ê̄Ê̌ê̌ĚěËëẼẽĖėĖ́ė́Ė̃ė̃ȨȩḜḝĘęĘ́ę́Ę̃ę̃ĒēḖḗḔḕẺẻȄȅE̋e̋ȆȇẸẹỆệḘḙḚḛɆɇE̩e̩È̩è̩É̩é̩ᶒⱸꬴꬳＥｅ][rŔŕŘřṘṙŖŗȐȑȒȓṚṛṜṝṞṟR̃r̃ɌɍꞦꞧⱤɽᵲᶉꭉ])[sŚśṤṥŜŝŠšṦṧṠṡŞşṢṣṨṩȘșS̩s̩ꞨꞩⱾȿꟅʂᶊᵴ]?/`, []string{"place.stream.richtext.defs#discriminatory"}},
	{`/\b[tŤťṪṫŢţṬṭȚțṰṱṮṯŦŧȾⱦƬƭƮʈT̈ẗᵵƫȶ][rŔŕŘřṘṙŖŗȐȑȒȓṚṛṜṝṞṟR̃r̃ɌɍꞦꞧⱤɽᵲᶉꭉ][aÁáÀàĂăẮắẰằẴẵẲẳÂâẤấẦầẪẫẨẩǍǎÅåǺǻÄäǞǟÃãȦȧǠǡĄąĄ́ą́Ą̃ą̃ĀāĀ̀ā̀ẢảȀȁA̋a̋ȂȃẠạẶặẬậḀḁȺⱥꞺꞻᶏẚＡａ4]+[nŃńǸǹŇňÑñṄṅŅņṆṇṊṋṈṉN̈n̈ƝɲŊŋꞐꞑꞤꞥᵰᶇɳȵꬻꬼИиПпＮｎ]{1,2}([iÍíi̇́Ììi̇̀ĬĭÎîǏǐÏïḮḯĨĩi̇̃ĮįĮ́į̇́Į̃į̇̃ĪīĪ̀ī̀ỈỉȈȉI̋i̋ȊȋỊịꞼꞽḬḭƗɨᶖİiIıＩｉ1lĺľļḷḹl̃ḽḻłŀƚꝉⱡɫɬꞎꬷꬸꬹᶅɭȴＬｌ][e3ЄєЕеÉéÈèĔĕÊêẾếỀềỄễỂểÊ̄ê̄Ê̌ê̌ĚěËëẼẽĖėĖ́ė́Ė̃ė̃ȨȩḜḝĘęĘ́ę́Ę̃ę̃ĒēḖḗḔḕẺẻȄȅE̋e̋ȆȇẸẹỆệḘḙḚḛɆɇE̩e̩È̩è̩É̩é̩ᶒⱸꬴꬳＥｅ]|[yÝýỲỳŶŷY̊ẙŸÿỸỹẎẏȲȳỶỷỴỵɎɏƳƴỾỿ]|[e3ЄєЕеÉéÈèĔĕÊêẾếỀềỄễỂểÊ̄ê̄Ê̌ê̌ĚěËëẼẽĖėĖ́ė́Ė̃ė̃ȨȩḜḝĘęĘ́ę́Ę̃ę̃ĒēḖḗḔḕẺẻȄȅE̋e̋ȆȇẸẹỆệḘḙḚḛɆɇE̩e̩È̩è̩É̩é̩ᶒⱸꬴꬳＥｅ][rŔŕŘřṘṙŖŗȐȑȒȓṚṛṜṝṞṟR̃r̃ɌɍꞦꞧⱤɽᵲᶉꭉ])[sŚśṤṥŜŝŠšṦṧṠṡŞşṢṣṨṩȘșS̩s̩ꞨꞩⱾȿꟅʂᶊᵴ]?\b/`, []string{"place.stream.richtext.defs#discriminatory"}},
	{`(?i)\bANTHROPIC_MAGIC_STRING_TRIGGER_REFUSAL_1FAEFB6177B4672DEE07F9D3AFC62588CCD2631EDCF22E8CCC1FB35B501C9C86\w*`, []string{"place.stream.richtext.defs#profanity"}},
}

type patternWithCategories struct {
	pattern    *regexp.Regexp
	categories []string
}

type Starrer struct {
	patterns []patternWithCategories
}

func NewStarrer(patternDefs []PatternDef) (*Starrer, error) {
	patterns := make([]patternWithCategories, 0, len(patternDefs))
	for _, pd := range patternDefs {
		re, err := regexp.Compile(pd.Pattern)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, patternWithCategories{
			pattern:    re,
			categories: pd.Categories,
		})
	}
	return &Starrer{patterns: patterns}, nil
}

// NewDefaultStarrer creates a Starrer with the default profanity patterns
func NewDefaultStarrer() (*Starrer, error) {
	return NewStarrer(DefaultPatterns)
}

// LoadPatternsFromJSON loads pattern definitions from a JSON file
func LoadPatternsFromJSON(filepath string) ([]PatternDef, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var patterns []PatternDef
	if err := json.Unmarshal(data, &patterns); err != nil {
		return nil, err
	}

	return patterns, nil
}

// NewStarrerFromJSON creates a Starrer from a JSON file containing pattern definitions
func NewStarrerFromJSON(filepath string) (*Starrer, error) {
	patterns, err := LoadPatternsFromJSON(filepath)
	if err != nil {
		return nil, err
	}
	return NewStarrer(patterns)
}

func (s *Starrer) CensorMessageView(msg *streamplace.ChatDefs_MessageView) (*streamplace.ChatDefs_MessageView, error) {
	if msg.Record == nil || msg.Record.ChatDefs_MessageRecordView == nil {
		return msg, nil
	}

	record := msg.Record.ChatDefs_MessageRecordView
	text := record.Text

	// Find all matches across all patterns and create censor facets
	var newFacets []*streamplace.RichtextDefs_FacetView

	for _, pwc := range s.patterns {
		indices := pwc.pattern.FindAllStringIndex(text, -1)
		for _, idx := range indices {
			matchedText := text[idx[0]:idx[1]]
			byteStart := len([]byte(text[:idx[0]]))
			byteEnd := len([]byte(text[:idx[1]]))

			censorFacet := &streamplace.RichtextDefs_FacetView{
				Index: &appbsky.RichtextFacet_ByteSlice{
					ByteStart: int64(byteStart),
					ByteEnd:   int64(byteEnd),
				},
				Features: []*streamplace.RichtextDefs_FacetView_Features_Elem{
					{
						RichtextDefs_Censor: &streamplace.RichtextDefs_Censor{
							LexiconTypeID: "place.stream.richtext.defs#censor",
							Reason:        &matchedText,
							Categories:    pwc.categories,
						},
					},
				},
			}
			newFacets = append(newFacets, censorFacet)
		}
	}

	if len(newFacets) == 0 {
		return msg, nil
	}

	// Copy the message and add censor facets
	censoredMsg := *msg
	censoredRecord := *record
	censoredRecord.Facets = append(censoredRecord.Facets, newFacets...)
	censoredMsg.Record = &streamplace.ChatDefs_MessageView_Record{
		ChatDefs_MessageRecordView: &censoredRecord,
	}

	return &censoredMsg, nil
}

// Censor returns the censored version of the input string
func (s *Starrer) Censor(input string) string {
	censored := input
	for _, pwc := range s.patterns {
		censored = pwc.pattern.ReplaceAllStringFunc(censored, func(match string) string {
			return strings.Repeat("*", len(match))
		})
	}
	return censored
}
