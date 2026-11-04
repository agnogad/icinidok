import { MainColumn, PageShell } from "../../components/Frame";
import ProfileCard from "../../components/ProfileCard";

export const metadata = {
  title: "Profilim — içini dök.",
  description: "Takma adın, paylaşımların ve anonim hesabın.",
};

export default function BenPage() {
  return (
    <PageShell active="ben">
      <MainColumn>
        <div className="scroll-area">
          <div className="simple-container">
            <ProfileCard />
          </div>
        </div>
      </MainColumn>
    </PageShell>
  );
}
