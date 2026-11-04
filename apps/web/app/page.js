import HomeWall from "../components/HomeWall";
import { MainColumn, PageShell } from "../components/Frame";

export const metadata = {
  title: "içini dök. — İçinde kalmasın.",
  description:
    "Derdini anonim olarak paylaş; okuyan biriyle 24 saatlik sakin bir sohbet aç.",
};

export default function HomePage() {
  return (
    <PageShell active="home">
      <MainColumn>
        <div className="scroll-area">
          <HomeWall />
        </div>
      </MainColumn>
    </PageShell>
  );
}
