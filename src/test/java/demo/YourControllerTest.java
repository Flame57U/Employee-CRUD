package demo;


import org.example.demo.controller.EmployeeController;
import org.junit.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.test.web.servlet.MockMvc;

@WebMvcTest(EmployeeController.class)
public class YourControllerTest {

    @Autowired
    private MockMvc mockMvc;

    @Test
    public void testEmployeeController() throws Exception{
        mockMvc.perform(get("/save"))
                .andExpect(status().isok())
                .andExpect(content().xontentTpye(MediaType.APPLICATION_JSON))
                .andExpect(jsonPath("$.data").exists);
    }
}
