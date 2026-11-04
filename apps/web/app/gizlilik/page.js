import Link from "next/link";
import { MainColumn, PageShell } from "../../components/Frame";

export const metadata = {
  title: "Gizlilik ve veri kullanımı — içini dök.",
  description: "içini dök. gizlilik ve veri saklama yaklaşımı.",
};

export default function GizlilikPage() {
  return (
    <PageShell active="hakkinda">
      <MainColumn>
        <div className="scroll-area">
          <div className="simple-container">
            <div className="simple-card">
              <h1>Gizlilik ve veri kullanımı</h1>
              <p>
                içini dök. takma adla yazmayı esas alır. Hesap açarken yalnızca
                bir kullanıcı adı seçersin; e-posta, telefon ve şifre istenmez.
                Giriş, authenticator uygulamasındaki 6 haneli kodla veya daha
                önce aldığın tek kullanımlık kurtarma kodlarıyla yapılır.
              </p>
              <p>
                Paylaşımlarının başlığı ve kısa önizlemesi duvarda, detayın
                tamamı paylaşım sayfasında herkese açık görünür. Özel mesajlar,
                kaydettiklerin ve katkı bakiyen herkese açık değildir.
              </p>
              <p>
                Sohbetin bitmesi mesajları silmez. Gelen kutunda en fazla 50
                sohbet tutulur; süre dolan sohbetler de yer tutar. “Listemden
                kaldır” yalnızca konuşmayı senin listenden kaldırır.
              </p>
              <Link className="simple-back" href="/hakkinda">
                Yardım ve güvenliğe dön
              </Link>
            </div>
          </div>
        </div>
      </MainColumn>
    </PageShell>
  );
}
