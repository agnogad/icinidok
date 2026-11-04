import { MainColumn, PageShell } from "../../components/Frame";

export const metadata = {
  title: "Yardım ve güvenlik — içini dök.",
  description:
    "içini dök. nasıl çalışır, güvenlik, acil durumlar, misafir oturumu ve veri saklama.",
};

const sections = [
  {
    id: "nedir",
    title: "Burası ne?",
    body: (
      <p>
        içini dök. içindekini anonim olarak yazabildiğin ve okuyan biriyle kısa,
        yazılı bir sohbet açabildiğin bir alandır. Burası terapi, danışmanlık ya
        da profesyonel destek hizmeti <strong>değildir</strong>.
      </p>
    ),
  },
  {
    id: "nasil",
    title: "Nasıl çalışır?",
    body: (
      <ul>
        <li>
          Paylaşımın başlığı ve detayının kısa önizlemesi duvarda, detayın tamamı
          paylaşım sayfasında herkese açık görünür.
        </li>
        <li>
          Takma adına tıklayanlar herkese açık profilindeki yayında olan tüm
          dertlerini görebilir. Özel mesajların, kaydettiklerin ve katkı bakiyen
          herkese açık değildir.
        </li>
        <li>
          Okuyan biri sana tek bir ilk mesaj gönderebilir; bu mesaj doğrudan gelen
          kutuna düşer. Ayrı bir kabul adımı yoktur.
        </li>
        <li>
          Sen cevap verdiğin anda 24 saatlik sohbet başlar. Süre sunucuda tutulur;
          sayfayı yenilemek veya sekme değiştirmek süreyi uzatmaz.
        </li>
        <li>
          Süre dolunca iki taraf da yeni mesaj yazamaz ve aynı oda yeniden
          açılmaz.
        </li>
        <li>
          Cevaplanmayan ilk mesajlar 24 saat sonra kapanır (bu süre ürün
          varsayımıdır, değişebilir).
        </li>
      </ul>
    ),
  },
  {
    id: "guvenlik",
    title: "Güvenlik",
    body: (
      <ul>
        <li>
          Sohbeti istediğin an bitirebilir, karşı tarafı engelleyebilir veya
          bildirebilirsin. Bu araçlar herkes için ücretsizdir.
        </li>
        <li>Engellediğin kişi sana yeni bir konuşmadan da yazamaz.</li>
        <li>
          İsim, telefon, adres, okul ya da iş yeri gibi tanımlayıcı bilgileri
          paylaşmamanı öneririz.
        </li>
        <li>
          Hizmet 18 yaş ve üzeri kullanıcılar için tasarlanmıştır. Bu bir beyan
          esasıdır; yaş doğrulaması yapılmaz.
        </li>
      </ul>
    ),
  },
  {
    id: "acil",
    title: "Acil bir durumdaysan",
    body: (
      <p>
        Kendin ya da bir başkası için hayati bir tehlike varsa burada beklemeden
        yerel acil yardım hattını ara. Türkiye&apos;de acil çağrı numarası{" "}
        <strong>112</strong>&apos;dir. Bir ruh sağlığı uzmanına veya güvendiğin
        birine ulaşmak da iyi bir adımdır.
      </p>
    ),
  },
  {
    id: "misafir",
    title: "Anonim hesap",
    body: (
      <p>
        Hesap açmadan dertleri okuyabilirsin. Yazmak ve sohbetlere devam etmek
        için yalnızca bir kullanıcı adı seçmen yeterli. E-posta, telefon ve şifre
        istemiyoruz. Giriş, authenticator uygulamasındaki 6 haneli kodla veya
        kayıt sırasında aldığın tek kullanımlık kurtarma kodlarıyla yapılır.
        Kurtarma kodlarını kaybetme; e-posta ile sıfırlama yoktur.
      </p>
    ),
  },
  {
    id: "veri",
    title: "Veri ve saklama",
    body: (
      <p>
        Sohbetin bitmesi mesajları silmez. Gelen kutunda en fazla 50 sohbet
        tutabilirsin; süre dolan sohbetler de yer tutar. “Listemden kaldır”
        konuşmayı kapatır ve yalnızca senin listenden kaldırır; mesajları kalıcı
        olarak silmez.{" "}
        <a href="/gizlilik">Gizlilik ve veri kullanımı hakkında ayrıntılar.</a>
      </p>
    ),
  },
];

export default function HakkindaPage() {
  return (
    <PageShell active="hakkinda">
      <MainColumn>
        <div className="scroll-area">
          <div className="help-container">
            <h1>Yardım ve güvenlik</h1>
            <nav aria-label="Bölümler" className="help-chips">
              {sections.map((section) => (
                <a key={section.id} href={`#${section.id}`}>
                  {section.title}
                </a>
              ))}
            </nav>

            <div className="help-sections">
              {sections.map((section) => (
                <section
                  key={section.id}
                  id={section.id}
                  className="help-section"
                >
                  <h2>{section.title}</h2>
                  <div className="help-section-body">{section.body}</div>
                </section>
              ))}
            </div>

            <footer className="help-footer">
              <a href="/">Duvara dön</a>
              <span>Yapan: BEYLER STUDIO</span>
            </footer>
          </div>
        </div>
      </MainColumn>
    </PageShell>
  );
}
