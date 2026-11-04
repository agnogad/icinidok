import { MainColumn, PageShell } from "../../components/Frame";
import PaylasForm from "../../components/PaylasForm";

export const metadata = {
  title: "Bir şey paylaş — içini dök.",
  description:
    "İçini dök, bir şey anlat veya aradığını paylaş. Takma adın dışında kimlik bilgin görünmez.",
};

export default function PaylasPage() {
  return (
    <PageShell active="paylas">
      <MainColumn>
        <div className="scroll-area">
          <PaylasForm />
        </div>
      </MainColumn>
    </PageShell>
  );
}
